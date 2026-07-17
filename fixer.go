package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const targetBaseURL = "https://api.x.ai/v1"

type authStatus struct {
	AuthIndex            string `json:"auth_index"`
	Name                 string `json:"name"`
	Email                string `json:"email,omitempty"`
	Disabled             bool   `json:"disabled"`
	BaseURL              string `json:"base_url,omitempty"`
	UsingAPI             bool   `json:"using_api"`
	HasUsingAPI          bool   `json:"has_using_api"`
	NeedsFix             bool   `json:"needs_fix"`
	LastOperation        string `json:"last_operation,omitempty"`
	Error                string `json:"error,omitempty"`
}

type statusSnapshot struct {
	Busy       bool         `json:"busy"`
	Operation  string       `json:"operation,omitempty"`
	Done       int          `json:"done"`
	Total      int          `json:"total"`
	ScannedAt  string       `json:"scanned_at,omitempty"`
	LastError  string       `json:"last_error,omitempty"`
	TargetURL  string       `json:"target_url"`
	Accounts   []authStatus `json:"accounts"`
}

type fixRequest struct {
	AuthIndexes []string `json:"auth_indexes"`
}

type fixerEngine struct {
	mu         sync.Mutex
	busy       bool
	operation  string
	done       int
	total      int
	scannedAt  string
	lastError  string
	accounts   []authStatus
}

var fixer = &fixerEngine{}

func (f *fixerEngine) snapshot() statusSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return statusSnapshot{
		Busy: f.busy, Operation: f.operation, Done: f.done, Total: f.total,
		ScannedAt: f.scannedAt, LastError: f.lastError,
		TargetURL: targetBaseURL, Accounts: append([]authStatus(nil), f.accounts...),
	}
}

func (f *fixerEngine) startScan() error {
	f.mu.Lock()
	if f.busy {
		f.mu.Unlock()
		return fmt.Errorf("another operation is already running")
	}
	f.busy, f.operation, f.done, f.total, f.lastError = true, "scan", 0, 0, ""
	f.mu.Unlock()
	go f.runScan()
	return nil
}

func (f *fixerEngine) runScan() {
	accounts, err := scanAccounts(func(done, total int) {
		f.mu.Lock()
		f.done, f.total = done, total
		f.mu.Unlock()
	})
	f.mu.Lock()
	defer f.mu.Unlock()
	f.busy = false
	f.operation = ""
	if err != nil {
		f.lastError = err.Error()
		return
	}
	f.accounts = accounts
	f.done, f.total = len(accounts), len(accounts)
	f.scannedAt = time.Now().Format(time.RFC3339)
}

func (f *fixerEngine) startFix(req fixRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.busy {
		return fmt.Errorf("another operation is already running")
	}
	selected := make(map[string]struct{}, len(req.AuthIndexes))
	for _, value := range req.AuthIndexes {
		if value = strings.TrimSpace(value); value != "" {
			selected[value] = struct{}{}
		}
	}
	var targets []authStatus
	for _, account := range f.accounts {
		if !account.NeedsFix || account.AuthIndex == "" {
			continue
		}
		if len(selected) > 0 {
			if _, ok := selected[account.AuthIndex]; !ok {
				continue
			}
		}
		targets = append(targets, account)
	}
	if len(targets) == 0 {
		return fmt.Errorf("no repairable xAI auth files selected")
	}
	f.busy, f.operation, f.done, f.total, f.lastError = true, "fix", 0, len(targets), ""
	go f.runFix(targets)
	return nil
}

func (f *fixerEngine) runFix(targets []authStatus) {
	for i, target := range targets {
		err := repairAccount(target)
		f.mu.Lock()
		for n := range f.accounts {
			if f.accounts[n].AuthIndex == target.AuthIndex {
				if err != nil {
					f.accounts[n].Error = err.Error()
					f.accounts[n].LastOperation = "failed"
				} else {
					f.accounts[n].BaseURL = targetBaseURL
					f.accounts[n].UsingAPI = true
					f.accounts[n].HasUsingAPI = true
					f.accounts[n].NeedsFix = false
					f.accounts[n].Error = ""
					f.accounts[n].LastOperation = "fixed"
				}
				break
			}
		}
		f.done = i + 1
		f.mu.Unlock()
	}
	f.mu.Lock()
	f.busy = false
	f.operation = ""
	f.scannedAt = time.Now().Format(time.RFC3339)
	f.mu.Unlock()
}

func (f *fixerEngine) startRollback(req fixRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.busy {
		return fmt.Errorf("another operation is already running")
	}
	selected := make(map[string]struct{}, len(req.AuthIndexes))
	for _, value := range req.AuthIndexes {
		if value = strings.TrimSpace(value); value != "" {
			selected[value] = struct{}{}
		}
	}
	var targets []authStatus
	for _, account := range f.accounts {
		if account.AuthIndex == "" {
			continue
		}
		if len(selected) > 0 {
			if _, ok := selected[account.AuthIndex]; !ok {
				continue
			}
		}
		targets = append(targets, account)
	}
	if len(targets) == 0 {
		return fmt.Errorf("no xAI auth files selected")
	}
	f.busy, f.operation, f.done, f.total, f.lastError = true, "rollback", 0, len(targets), ""
	go f.runRollback(targets)
	return nil
}

type restoredFields struct {
	BaseURL     string
	UsingAPI    bool
	HasUsingAPI bool
}

func (f *fixerEngine) runRollback(targets []authStatus) {
	for i, target := range targets {
		restored, err := rollbackAccount(target)
		f.mu.Lock()
		for n := range f.accounts {
			if f.accounts[n].AuthIndex == target.AuthIndex {
				if err != nil {
					f.accounts[n].Error = err.Error()
					f.accounts[n].LastOperation = "rollback_failed"
				} else {
					f.accounts[n].BaseURL = restored.BaseURL
					f.accounts[n].UsingAPI = restored.UsingAPI
					f.accounts[n].HasUsingAPI = restored.HasUsingAPI
					f.accounts[n].NeedsFix = restored.BaseURL != targetBaseURL || !restored.HasUsingAPI || !restored.UsingAPI
					f.accounts[n].Error = ""
					f.accounts[n].LastOperation = "rolled_back"
				}
				break
			}
		}
		f.done = i + 1
		f.mu.Unlock()
	}
	f.mu.Lock()
	f.busy = false
	f.operation = ""
	f.scannedAt = time.Now().Format(time.RFC3339)
	f.mu.Unlock()
}

func scanAccounts(progress func(int, int)) ([]authStatus, error) {
	raw, err := callHost(methodHostAuthList, map[string]any{})
	if err != nil {
		return nil, err
	}
	var list authListResponse
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decode host.auth.list: %w", err)
	}
	var candidates []authFileEntry
	for _, file := range list.Files {
		provider := strings.ToLower(strings.TrimSpace(firstNonEmpty(file.Provider, file.Type)))
		if provider == "xai" || strings.HasPrefix(strings.ToLower(file.Name), "xai-") {
			candidates = append(candidates, file)
		}
	}
	progress(0, len(candidates))
	accounts := make([]authStatus, 0, len(candidates))
	for i, file := range candidates {
		accounts = append(accounts, inspectAuth(file))
		progress(i+1, len(candidates))
	}
	sort.Slice(accounts, func(i, j int) bool { return strings.ToLower(accounts[i].Name) < strings.ToLower(accounts[j].Name) })
	return accounts, nil
}

func inspectAuth(file authFileEntry) authStatus {
	status := authStatus{AuthIndex: file.AuthIndex, Name: file.Name, Email: file.Email, Disabled: file.Disabled}
	if file.AuthIndex == "" {
		status.Error = "auth_index is unavailable"
		return status
	}
	resp, data, err := getAuth(file.AuthIndex)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	if resp.Name != "" {
		status.Name = resp.Name
	}
	if typ := strings.ToLower(strings.TrimSpace(asString(data["type"]))); typ != "xai" {
		status.Error = "credential JSON type is not xai"
		return status
	}
	status.Email = firstNonEmpty(asString(data["email"]), status.Email)
	status.BaseURL = strings.TrimSpace(asString(data["base_url"]))
	status.UsingAPI, status.HasUsingAPI = data["using_api"].(bool)
	status.NeedsFix = status.BaseURL != targetBaseURL || !status.HasUsingAPI || !status.UsingAPI
	return status
}

func repairAccount(target authStatus) error {
	resp, data, err := getAuth(target.AuthIndex)
	if err != nil {
		return err
	}
	if strings.ToLower(strings.TrimSpace(asString(data["type"]))) != "xai" {
		return fmt.Errorf("refusing to modify non-xai credential")
	}
	name := filepath.Base(firstNonEmpty(resp.Name, target.Name))
	if name == "." || !strings.HasSuffix(strings.ToLower(name), ".json") {
		return fmt.Errorf("invalid auth file name")
	}
	data["base_url"] = targetBaseURL
	data["using_api"] = true
	updated, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode repaired auth: %w", err)
	}
	if _, err := callHost(methodHostAuthSave, authSaveRequest{Name: name, JSON: append(updated, '\n')}); err != nil {
		return fmt.Errorf("save repaired auth: %w", err)
	}
	return nil
}

func rollbackAccount(target authStatus) (restoredFields, error) {
	resp, data, err := getAuth(target.AuthIndex)
	if err != nil {
		return restoredFields{}, err
	}
	if strings.ToLower(strings.TrimSpace(asString(data["type"]))) != "xai" {
		return restoredFields{}, fmt.Errorf("refusing to modify non-xai credential")
	}
	name := filepath.Base(firstNonEmpty(resp.Name, target.Name))
	if name == "." || !strings.HasSuffix(strings.ToLower(name), ".json") {
		return restoredFields{}, fmt.Errorf("invalid auth file name")
	}
	data["base_url"] = "https://cli-chat-proxy.grok.com/v1"
	delete(data, "using_api")
	updated, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return restoredFields{}, fmt.Errorf("encode rolled back auth: %w", err)
	}
	if _, err := callHost(methodHostAuthSave, authSaveRequest{Name: name, JSON: append(updated, '\n')}); err != nil {
		return restoredFields{}, fmt.Errorf("save rolled back auth: %w", err)
	}
	return restoredFields{BaseURL: "https://cli-chat-proxy.grok.com/v1"}, nil
}

func getAuth(authIndex string) (authGetResponse, map[string]any, error) {
	raw, err := callHost(methodHostAuthGet, authGetRequest{AuthIndex: authIndex})
	if err != nil {
		return authGetResponse{}, nil, err
	}
	var resp authGetResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return resp, nil, fmt.Errorf("decode host.auth.get: %w", err)
	}
	var data map[string]any
	if err := json.Unmarshal(resp.JSON, &data); err != nil {
		return resp, nil, fmt.Errorf("decode auth JSON: %w", err)
	}
	return resp, data, nil
}

func asString(value any) string {
	text, _ := value.(string)
	return text
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
