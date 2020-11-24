package transit

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/mitchellh/mapstructure"
)

// Case1: Ensure that batch encryption did not affect the normal flow of
// encrypting the plaintext with a pre-existing key.
func TestTransit_BatchEncryptionCase1(t *testing.T) {
	var resp *logical.Response
	var err error

	b, s := createBackendWithStorage(t)

	// Create the policy
	policyReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/existing_key",
		Storage:   s,
	}
	resp, err = b.HandleRequest(context.Background(), policyReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	plaintext := "dGhlIHF1aWNrIGJyb3duIGZveA==" // "the quick brown fox"

	encData := map[string]interface{}{
		"plaintext": plaintext,
	}

	encReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "encrypt/existing_key",
		Storage:   s,
		Data:      encData,
	}
	resp, err = b.HandleRequest(context.Background(), encReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	keyVersion := resp.Data["key_version"].(int)
	if keyVersion != 1 {
		t.Fatalf("unexpected key version; got: %d, expected: %d", keyVersion, 1)
	}

	ciphertext := resp.Data["ciphertext"]

	decData := map[string]interface{}{
		"ciphertext": ciphertext,
	}
	decReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "decrypt/existing_key",
		Storage:   s,
		Data:      decData,
	}
	resp, err = b.HandleRequest(context.Background(), decReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	if resp.Data["plaintext"] != plaintext {
		t.Fatalf("bad: plaintext. Expected: %q, Actual: %q", plaintext, resp.Data["plaintext"])
	}
}

// Case2: Ensure that batch encryption did not affect the normal flow of
// encrypting the plaintext with the key upserted.
func TestTransit_BatchEncryptionCase2(t *testing.T) {
	var resp *logical.Response
	var err error
	b, s := createBackendWithStorage(t)

	// Upsert the key and encrypt the data
	plaintext := "dGhlIHF1aWNrIGJyb3duIGZveA=="

	encData := map[string]interface{}{
		"plaintext": plaintext,
	}

	encReq := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "encrypt/upserted_key",
		Storage:   s,
		Data:      encData,
	}
	resp, err = b.HandleRequest(context.Background(), encReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	keyVersion := resp.Data["key_version"].(int)
	if keyVersion != 1 {
		t.Fatalf("unexpected key version; got: %d, expected: %d", keyVersion, 1)
	}

	ciphertext := resp.Data["ciphertext"]
	decData := map[string]interface{}{
		"ciphertext": ciphertext,
	}

	policyReq := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "keys/upserted_key",
		Storage:   s,
	}

	resp, err = b.HandleRequest(context.Background(), policyReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	decReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "decrypt/upserted_key",
		Storage:   s,
		Data:      decData,
	}
	resp, err = b.HandleRequest(context.Background(), decReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	if resp.Data["plaintext"] != plaintext {
		t.Fatalf("bad: plaintext. Expected: %q, Actual: %q", plaintext, resp.Data["plaintext"])
	}
}

// Case3: If batch encryption input is not base64 encoded, it should fail.
func TestTransit_BatchEncryptionCase3(t *testing.T) {
	var err error

	b, s := createBackendWithStorage(t)

	batchInput := `[{"plaintext":"dGhlIHF1aWNrIGJyb3duIGZveA=="}]`
	batchData := map[string]interface{}{
		"batch_input": batchInput,
	}

	batchReq := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "encrypt/upserted_key",
		Storage:   s,
		Data:      batchData,
	}
	_, err = b.HandleRequest(context.Background(), batchReq)
	if err == nil {
		t.Fatal("expected an error")
	}
}

// Case4: Test batch encryption with an existing key
func TestTransit_BatchEncryptionCase4(t *testing.T) {
	var resp *logical.Response
	var err error

	b, s := createBackendWithStorage(t)

	policyReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/existing_key",
		Storage:   s,
	}
	resp, err = b.HandleRequest(context.Background(), policyReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	batchInput := []interface{}{
		map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA=="},
		map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA=="},
	}

	batchData := map[string]interface{}{
		"batch_input": batchInput,
	}
	batchReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "encrypt/existing_key",
		Storage:   s,
		Data:      batchData,
	}
	resp, err = b.HandleRequest(context.Background(), batchReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	batchResponseItems := resp.Data["batch_results"].([]BatchResponseItem)

	decReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "decrypt/existing_key",
		Storage:   s,
	}

	plaintext := "dGhlIHF1aWNrIGJyb3duIGZveA=="

	for _, item := range batchResponseItems {
		if item.KeyVersion != 1 {
			t.Fatalf("unexpected key version; got: %d, expected: %d", item.KeyVersion, 1)
		}

		decReq.Data = map[string]interface{}{
			"ciphertext": item.Ciphertext,
		}
		resp, err = b.HandleRequest(context.Background(), decReq)
		if err != nil || (resp != nil && resp.IsError()) {
			t.Fatalf("err:%v resp:%#v", err, resp)
		}

		if resp.Data["plaintext"] != plaintext {
			t.Fatalf("bad: plaintext. Expected: %q, Actual: %q", plaintext, resp.Data["plaintext"])
		}
	}
}

// Case5: Test batch encryption with an existing derived key
func TestTransit_BatchEncryptionCase5(t *testing.T) {
	var resp *logical.Response
	var err error

	b, s := createBackendWithStorage(t)

	policyData := map[string]interface{}{
		"derived": true,
	}

	policyReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/existing_key",
		Storage:   s,
		Data:      policyData,
	}

	resp, err = b.HandleRequest(context.Background(), policyReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	batchInput := []interface{}{
		map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA==", "context": "dmlzaGFsCg=="},
		map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA==", "context": "dmlzaGFsCg=="},
	}

	batchData := map[string]interface{}{
		"batch_input": batchInput,
	}

	batchReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "encrypt/existing_key",
		Storage:   s,
		Data:      batchData,
	}
	resp, err = b.HandleRequest(context.Background(), batchReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	batchResponseItems := resp.Data["batch_results"].([]BatchResponseItem)

	decReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "decrypt/existing_key",
		Storage:   s,
	}

	plaintext := "dGhlIHF1aWNrIGJyb3duIGZveA=="

	for _, item := range batchResponseItems {
		if item.KeyVersion != 1 {
			t.Fatalf("unexpected key version; got: %d, expected: %d", item.KeyVersion, 1)
		}

		decReq.Data = map[string]interface{}{
			"ciphertext": item.Ciphertext,
			"context":    "dmlzaGFsCg==",
		}
		resp, err = b.HandleRequest(context.Background(), decReq)
		if err != nil || (resp != nil && resp.IsError()) {
			t.Fatalf("err:%v resp:%#v", err, resp)
		}

		if resp.Data["plaintext"] != plaintext {
			t.Fatalf("bad: plaintext. Expected: %q, Actual: %q", plaintext, resp.Data["plaintext"])
		}
	}
}

// Case6: Test batch encryption with an upserted non-derived key
func TestTransit_BatchEncryptionCase6(t *testing.T) {
	var resp *logical.Response
	var err error

	b, s := createBackendWithStorage(t)

	batchInput := []interface{}{
		map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA=="},
		map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA=="},
	}

	batchData := map[string]interface{}{
		"batch_input": batchInput,
	}
	batchReq := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "encrypt/upserted_key",
		Storage:   s,
		Data:      batchData,
	}
	resp, err = b.HandleRequest(context.Background(), batchReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	batchResponseItems := resp.Data["batch_results"].([]BatchResponseItem)

	decReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "decrypt/upserted_key",
		Storage:   s,
	}

	plaintext := "dGhlIHF1aWNrIGJyb3duIGZveA=="

	for _, responseItem := range batchResponseItems {
		var item BatchResponseItem
		if err := mapstructure.Decode(responseItem, &item); err != nil {
			t.Fatal(err)
		}

		if item.KeyVersion != 1 {
			t.Fatalf("unexpected key version; got: %d, expected: %d", item.KeyVersion, 1)
		}

		decReq.Data = map[string]interface{}{
			"ciphertext": item.Ciphertext,
		}
		resp, err = b.HandleRequest(context.Background(), decReq)
		if err != nil || (resp != nil && resp.IsError()) {
			t.Fatalf("err:%v resp:%#v", err, resp)
		}

		if resp.Data["plaintext"] != plaintext {
			t.Fatalf("bad: plaintext. Expected: %q, Actual: %q", plaintext, resp.Data["plaintext"])
		}
	}
}

// Case7: Test batch encryption with an upserted derived key
func TestTransit_BatchEncryptionCase7(t *testing.T) {
	var resp *logical.Response
	var err error

	b, s := createBackendWithStorage(t)

	batchInput := []interface{}{
		map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA==", "context": "dmlzaGFsCg=="},
		map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA==", "context": "dmlzaGFsCg=="},
	}

	batchData := map[string]interface{}{
		"batch_input": batchInput,
	}
	batchReq := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "encrypt/upserted_key",
		Storage:   s,
		Data:      batchData,
	}
	resp, err = b.HandleRequest(context.Background(), batchReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	batchResponseItems := resp.Data["batch_results"].([]BatchResponseItem)

	decReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "decrypt/upserted_key",
		Storage:   s,
	}

	plaintext := "dGhlIHF1aWNrIGJyb3duIGZveA=="

	for _, item := range batchResponseItems {
		if item.KeyVersion != 1 {
			t.Fatalf("unexpected key version; got: %d, expected: %d", item.KeyVersion, 1)
		}

		decReq.Data = map[string]interface{}{
			"ciphertext": item.Ciphertext,
			"context":    "dmlzaGFsCg==",
		}
		resp, err = b.HandleRequest(context.Background(), decReq)
		if err != nil || (resp != nil && resp.IsError()) {
			t.Fatalf("err:%v resp:%#v", err, resp)
		}

		if resp.Data["plaintext"] != plaintext {
			t.Fatalf("bad: plaintext. Expected: %q, Actual: %q", plaintext, resp.Data["plaintext"])
		}
	}
}

// Case8: If plaintext is not base64 encoded, encryption should fail
func TestTransit_BatchEncryptionCase8(t *testing.T) {
	var resp *logical.Response
	var err error

	b, s := createBackendWithStorage(t)

	// Create the policy
	policyReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/existing_key",
		Storage:   s,
	}
	resp, err = b.HandleRequest(context.Background(), policyReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	batchInput := []interface{}{
		map[string]interface{}{"plaintext": "simple_plaintext"},
	}
	batchData := map[string]interface{}{
		"batch_input": batchInput,
	}
	batchReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "encrypt/existing_key",
		Storage:   s,
		Data:      batchData,
	}
	resp, err = b.HandleRequest(context.Background(), batchReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	plaintext := "simple plaintext"

	encData := map[string]interface{}{
		"plaintext": plaintext,
	}

	encReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "encrypt/existing_key",
		Storage:   s,
		Data:      encData,
	}
	resp, err = b.HandleRequest(context.Background(), encReq)
	if err == nil {
		t.Fatal("expected an error")
	}
}

// Case9: If both plaintext and batch inputs are supplied, plaintext should be
// ignored.
func TestTransit_BatchEncryptionCase9(t *testing.T) {
	var resp *logical.Response
	var err error

	b, s := createBackendWithStorage(t)

	batchInput := []interface{}{
		map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA=="},
		map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA=="},
	}
	plaintext := "dGhlIHF1aWNrIGJyb3duIGZveA=="
	batchData := map[string]interface{}{
		"batch_input": batchInput,
		"plaintext":   plaintext,
	}
	batchReq := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "encrypt/upserted_key",
		Storage:   s,
		Data:      batchData,
	}
	resp, err = b.HandleRequest(context.Background(), batchReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("err:%v resp:%#v", err, resp)
	}

	_, ok := resp.Data["ciphertext"]
	if ok {
		t.Fatal("ciphertext field should not be set")
	}
}

// Case10: Inconsistent presence of 'context' in batch input should be caught
func TestTransit_BatchEncryptionCase10(t *testing.T) {
	var err error

	b, s := createBackendWithStorage(t)

	batchInput := []interface{}{
		map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA=="},
		map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA==", "context": "dmlzaGFsCg=="},
	}

	batchData := map[string]interface{}{
		"batch_input": batchInput,
	}

	batchReq := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "encrypt/upserted_key",
		Storage:   s,
		Data:      batchData,
	}
	_, err = b.HandleRequest(context.Background(), batchReq)
	if err == nil {
		t.Fatalf("expected an error")
	}
}

// Case11: Incorrect inputs for context and nonce should not fail the operation
func TestTransit_BatchEncryptionCase11(t *testing.T) {
	var err error

	b, s := createBackendWithStorage(t)

	batchInput := []interface{}{
		map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA==", "context": "dmlzaGFsCg=="},
		map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA==", "context": "not-encoded"},
	}

	batchData := map[string]interface{}{
		"batch_input": batchInput,
	}
	batchReq := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "encrypt/upserted_key",
		Storage:   s,
		Data:      batchData,
	}
	_, err = b.HandleRequest(context.Background(), batchReq)
	if err != nil {
		t.Fatal(err)
	}
}

// Case12: Invalid batch input
func TestTransit_BatchEncryptionCase12(t *testing.T) {
	var err error
	b, s := createBackendWithStorage(t)

	batchInput := []interface{}{
		map[string]interface{}{},
		"unexpected_interface",
	}

	batchData := map[string]interface{}{
		"batch_input": batchInput,
	}
	batchReq := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "encrypt/upserted_key",
		Storage:   s,
		Data:      batchData,
	}
	_, err = b.HandleRequest(context.Background(), batchReq)
	if err == nil {
		t.Fatalf("expected an error")
	}
}

// Test that the fast path function decodeBatchRequestItems behave like mapstructure.Decode() to decode []BatchRequestItem.
func TestTransit_decodeBatchRequestItems(t *testing.T) {
	tests := []struct {
		name string
		src  interface{}
		dest []BatchRequestItem
	}{
		// basic edge cases of nil values
		{name: "nil-nil", src: nil, dest: nil},
		{name: "nil-empty", src: nil, dest: []BatchRequestItem{}},
		{name: "empty-nil", src: []interface{}{}, dest: nil},
		{
			name: "src-nil",
			src:  []interface{}{map[string]interface{}{}},
			dest: nil,
		},
		// empty src & dest
		{
			name: "src-dest",
			src:  []interface{}{map[string]interface{}{}},
			dest: []BatchRequestItem{},
		},
		// empty src but with already populated dest, mapstructure discard pre-populated data.
		{
			name: "src-dest_pre_filled",
			src:  []interface{}{map[string]interface{}{}},
			dest: []BatchRequestItem{{}},
		},
		// two test per properties to test valid and invalid input
		{
			name: "src_plaintext-dest",
			src:  []interface{}{map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA=="}},
			dest: []BatchRequestItem{},
		},
		{
			name: "src_plaintext_invalid-dest",
			src:  []interface{}{map[string]interface{}{"plaintext": 666}},
			dest: []BatchRequestItem{},
		},
		{
			name: "src_ciphertext-dest",
			src:  []interface{}{map[string]interface{}{"ciphertext": "dGhlIHF1aWNrIGJyb3duIGZveA=="}},
			dest: []BatchRequestItem{},
		},
		{
			name: "src_ciphertext_invalid-dest",
			src:  []interface{}{map[string]interface{}{"ciphertext": 666}},
			dest: []BatchRequestItem{},
		},
		{
			name: "src_key_version-dest",
			src:  []interface{}{map[string]interface{}{"key_version": 1}},
			dest: []BatchRequestItem{},
		},
		{
			name: "src_key_version_invalid-dest",
			src:  []interface{}{map[string]interface{}{"key_version": "666"}},
			dest: []BatchRequestItem{},
		},
		{
			name: "src_nonce-dest",
			src:  []interface{}{map[string]interface{}{"nonce": "dGVzdGNvbnRleHQ="}},
			dest: []BatchRequestItem{},
		},
		{
			name: "src_nonce_invalid-dest",
			src:  []interface{}{map[string]interface{}{"nonce": 666}},
			dest: []BatchRequestItem{},
		},
		{
			name: "src_context-dest",
			src:  []interface{}{map[string]interface{}{"context": "dGVzdGNvbnRleHQ="}},
			dest: []BatchRequestItem{},
		},
		{
			name: "src_context_invalid-dest",
			src:  []interface{}{map[string]interface{}{"context": 666}},
			dest: []BatchRequestItem{},
		},
		{
			name: "src_multi_order-dest",
			src: []interface{}{
				map[string]interface{}{"context": "1"},
				map[string]interface{}{"context": "2"},
				map[string]interface{}{"context": "3"},
			},
			dest: []BatchRequestItem{},
		},
		{
			name: "src_multi_with_invalid-dest",
			src: []interface{}{
				map[string]interface{}{"context": "1"},
				map[string]interface{}{"context": "2", "key_version": "666"},
				map[string]interface{}{"context": "3"},
			},
			dest: []BatchRequestItem{},
		},
		{
			name: "src_multi_with_multi_invalid-dest",
			src: []interface{}{
				map[string]interface{}{"context": "1"},
				map[string]interface{}{"context": "2", "key_version": "666"},
				map[string]interface{}{"context": "3", "key_version": "1337"},
			},
			dest: []BatchRequestItem{},
		},
		{
			name: "src_plaintext-nil-nonce",
			src:  []interface{}{map[string]interface{}{"plaintext": "dGhlIHF1aWNrIGJyb3duIGZveA==", "nonce": "null"}},
			dest: []BatchRequestItem{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedDest := append(tt.dest[:0:0], tt.dest...) // copy of the dest state
			expectedErr := mapstructure.Decode(tt.src, &expectedDest)

			gotErr := decodeBatchRequestItems(tt.src, &tt.dest)
			gotDest := tt.dest

			if !reflect.DeepEqual(expectedErr, gotErr) {
				t.Errorf("decodeBatchRequestItems unexpected error value, want: '%v', got: '%v'", expectedErr, gotErr)
			}

			if !reflect.DeepEqual(expectedDest, gotDest) {
				t.Errorf("decodeBatchRequestItems unexpected dest value, want: '%v', got: '%v'", expectedDest, gotDest)
			}
		})
	}
}
