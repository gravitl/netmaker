package iru

import (
	"encoding/json"
	"testing"
)

func TestIruDevice_UnmarshalUserObject(t *testing.T) {
	var d iruDevice
	if err := json.Unmarshal([]byte(`{
		"device_id": "dev-1",
		"serial_number": "SN1",
		"user": {"email": "user@example.com", "name": "User One"}
	}`), &d); err != nil {
		t.Fatal(err)
	}
	if d.User.Email() != "user@example.com" {
		t.Fatalf("email = %q", d.User.Email())
	}
}

func TestIruDevice_UnmarshalUserString(t *testing.T) {
	var d iruDevice
	if err := json.Unmarshal([]byte(`{
		"device_id": "dev-1",
		"serial_number": "SN1",
		"user": "user@example.com"
	}`), &d); err != nil {
		t.Fatal(err)
	}
	if d.User.Email() != "user@example.com" {
		t.Fatalf("email = %q", d.User.Email())
	}
}

func TestIruDevice_UnmarshalUserNull(t *testing.T) {
	var d iruDevice
	if err := json.Unmarshal([]byte(`{"device_id":"dev-1","user":null}`), &d); err != nil {
		t.Fatal(err)
	}
	if d.User.Email() != "" {
		t.Fatalf("expected empty email, got %q", d.User.Email())
	}
}

func TestDecodeDeviceList_WrappedAndArray(t *testing.T) {
	wrapped := `{"devices":[{"device_id":"a","user":"a@b.com"}]}`
	var page devicesListResponse
	if err := json.Unmarshal([]byte(wrapped), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Devices) != 1 || page.Devices[0].User.Email() != "a@b.com" {
		t.Fatalf("unexpected wrapped decode: %+v", page.Devices)
	}

	array := `[{"device_id":"b","user":{"email":"b@b.com"}}]`
	var devices []iruDevice
	if err := json.Unmarshal([]byte(array), &devices); err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].User.Email() != "b@b.com" {
		t.Fatalf("unexpected array decode: %+v", devices)
	}
}
