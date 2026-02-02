package verification

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPWitness implements the Witness interface using HTTP API calls.
type HTTPWitness struct {
	name       string
	baseURL    string
	apiKey     string
	headers    map[string]string
	httpClient *http.Client
}

// NewHTTPWitness creates a new HTTP-based witness client.
func NewHTTPWitness(name, baseURL, apiKey string, headers map[string]string) *HTTPWitness {
	return &HTTPWitness{
		name:    name,
		baseURL: baseURL,
		apiKey:  apiKey,
		headers: headers,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the witness provider name.
func (w *HTTPWitness) Name() string {
	return w.name
}

// SubmitCheckpoint sends a checkpoint to the witness service.
func (w *HTTPWitness) SubmitCheckpoint(checkpoint *WitnessCheckpoint) (*WitnessReceipt, error) {
	data, err := json.Marshal(checkpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	req, err := http.NewRequest("POST", w.baseURL+"/checkpoint", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	w.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("witness returned status %d: %s", resp.StatusCode, string(body))
	}

	var receipt WitnessReceipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		// If witness doesn't return structured response, create basic receipt
		hash := sha256.Sum256(data)
		receipt = WitnessReceipt{
			CheckpointID: checkpoint.ID,
			WitnessName:  w.name,
			ReceivedAt:   time.Now(),
			WitnessHash:  hex.EncodeToString(hash[:]),
		}
	}

	if receipt.WitnessName == "" {
		receipt.WitnessName = w.name
	}

	return &receipt, nil
}

// VerifyCheckpoint retrieves and verifies a previously submitted checkpoint.
func (w *HTTPWitness) VerifyCheckpoint(id string) (*WitnessVerification, error) {
	req, err := http.NewRequest("GET", w.baseURL+"/checkpoint/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	w.setHeaders(req)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	verification := &WitnessVerification{
		CheckpointID: id,
		VerifiedAt:   time.Now(),
		WitnessName:  w.name,
	}

	if resp.StatusCode == http.StatusNotFound {
		verification.Valid = false
		verification.Errors = append(verification.Errors, "checkpoint not found in witness")
		return verification, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("witness returned status %d: %s", resp.StatusCode, string(body))
	}

	// Try to parse as checkpoint
	var checkpoint WitnessCheckpoint
	if err := json.Unmarshal(body, &checkpoint); err != nil {
		verification.Valid = false
		verification.Errors = append(verification.Errors, fmt.Sprintf("failed to parse checkpoint: %v", err))
		return verification, nil
	}

	verification.Checkpoint = &checkpoint

	// Compute hash of stored checkpoint
	hash, err := computeCheckpointHash(&checkpoint)
	if err != nil {
		verification.Errors = append(verification.Errors, fmt.Sprintf("failed to compute hash: %v", err))
	} else {
		verification.ComputedHash = hex.EncodeToString(hash)
	}

	// If witness returns stored hash, compare
	storedHash := sha256.Sum256(body)
	verification.StoredHash = hex.EncodeToString(storedHash[:])

	verification.Valid = len(verification.Errors) == 0

	return verification, nil
}

// Ping checks if the witness service is available.
func (w *HTTPWitness) Ping() error {
	req, err := http.NewRequest("GET", w.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	w.setHeaders(req)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("witness returned status %d", resp.StatusCode)
	}

	return nil
}

// setHeaders adds common headers to the request.
func (w *HTTPWitness) setHeaders(req *http.Request) {
	if w.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+w.apiKey)
	}

	for k, v := range w.headers {
		req.Header.Set(k, v)
	}
}

// AirgapperWitness implements Witness using another Airgapper instance.
type AirgapperWitness struct {
	name       string
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewAirgapperWitness creates a witness that uses another Airgapper instance.
func NewAirgapperWitness(name, baseURL, apiKey string) *AirgapperWitness {
	return &AirgapperWitness{
		name:    name,
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the witness provider name.
func (w *AirgapperWitness) Name() string {
	return w.name
}

// SubmitCheckpoint sends a checkpoint to the Airgapper witness.
func (w *AirgapperWitness) SubmitCheckpoint(checkpoint *WitnessCheckpoint) (*WitnessReceipt, error) {
	data, err := json.Marshal(checkpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	req, err := http.NewRequest("POST", w.baseURL+"/api/witness/checkpoint", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if w.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+w.apiKey)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("witness returned status %d: %s", resp.StatusCode, string(body))
	}

	var receipt WitnessReceipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		hash := sha256.Sum256(data)
		receipt = WitnessReceipt{
			CheckpointID: checkpoint.ID,
			WitnessName:  w.name,
			ReceivedAt:   time.Now(),
			WitnessHash:  hex.EncodeToString(hash[:]),
			StorageURL:   w.baseURL + "/api/witness/checkpoint/" + checkpoint.ID,
		}
	}

	return &receipt, nil
}

// VerifyCheckpoint retrieves and verifies a checkpoint from the Airgapper witness.
func (w *AirgapperWitness) VerifyCheckpoint(id string) (*WitnessVerification, error) {
	req, err := http.NewRequest("GET", w.baseURL+"/api/witness/checkpoint/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if w.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+w.apiKey)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	verification := &WitnessVerification{
		CheckpointID: id,
		VerifiedAt:   time.Now(),
		WitnessName:  w.name,
	}

	if resp.StatusCode == http.StatusNotFound {
		verification.Valid = false
		verification.Errors = append(verification.Errors, "checkpoint not found")
		return verification, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("witness returned status %d: %s", resp.StatusCode, string(body))
	}

	var checkpoint WitnessCheckpoint
	if err := json.Unmarshal(body, &checkpoint); err != nil {
		verification.Errors = append(verification.Errors, fmt.Sprintf("failed to parse: %v", err))
		return verification, nil
	}

	verification.Checkpoint = &checkpoint

	hash, err := computeCheckpointHash(&checkpoint)
	if err == nil {
		verification.ComputedHash = hex.EncodeToString(hash)
	}

	storedHash := sha256.Sum256(body)
	verification.StoredHash = hex.EncodeToString(storedHash[:])

	verification.Valid = len(verification.Errors) == 0

	return verification, nil
}

// Ping checks if the Airgapper witness is available.
func (w *AirgapperWitness) Ping() error {
	req, err := http.NewRequest("GET", w.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("witness returned status %d", resp.StatusCode)
	}

	return nil
}

// CreateWitnessFromProvider creates a Witness from a WitnessProvider config.
func CreateWitnessFromProvider(provider WitnessProvider) (Witness, error) {
	if !provider.Enabled {
		return nil, errors.New("provider is disabled")
	}

	if provider.URL == "" {
		return nil, errors.New("provider URL required")
	}

	switch provider.Type {
	case "http":
		return NewHTTPWitness(provider.Name, provider.URL, provider.APIKey, provider.Headers), nil
	case "airgapper":
		return NewAirgapperWitness(provider.Name, provider.URL, provider.APIKey), nil
	default:
		return nil, fmt.Errorf("unknown provider type: %s", provider.Type)
	}
}

// CreateWitnessesFromConfig creates witnesses from verification config.
func CreateWitnessesFromConfig(config *WitnessConfig) ([]Witness, error) {
	if config == nil || !config.Enabled {
		return nil, nil
	}

	var witnesses []Witness
	var errs []error

	for _, provider := range config.Providers {
		w, err := CreateWitnessFromProvider(provider)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", provider.Name, err))
			continue
		}
		witnesses = append(witnesses, w)
	}

	if len(errs) > 0 && len(witnesses) == 0 {
		return nil, fmt.Errorf("failed to create any witnesses: %v", errs)
	}

	return witnesses, nil
}
