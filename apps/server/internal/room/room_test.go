package room

import (
	"regexp"
	"testing"
)

func TestGenerateCodeHasSixDigits(t *testing.T) {
	code, err := generateCode()
	if err != nil {
		t.Fatalf("generateCode: %v", err)
	}
	if !regexp.MustCompile(`^\d{6}$`).MatchString(code) {
		t.Fatalf("code = %q, want six digits", code)
	}
}

func TestGenerateCodeIsNotConstant(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		code, err := generateCode()
		if err != nil {
			t.Fatalf("generateCode: %v", err)
		}
		if seen[code] {
			t.Fatalf("code %q was generated twice", code)
		}
		seen[code] = true
	}
}

func TestRegistryCreateMakesDistinctRooms(t *testing.T) {
	reg := NewRegistry(10, 10)

	first, err := reg.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	second, err := reg.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if first.Code() == second.Code() {
		t.Fatal("two rooms share the same code")
	}
	if reg.Count() != 2 {
		t.Fatalf("Count = %d, want 2", reg.Count())
	}
}

func TestRegistryGetFindsAnExistingRoom(t *testing.T) {
	reg := NewRegistry(10, 10)
	created, err := reg.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, ok := reg.Get(created.Code())
	if !ok {
		t.Fatal("Get did not find the room")
	}
	if found.Code() != created.Code() {
		t.Fatalf("code = %q, want %q", found.Code(), created.Code())
	}
}

// TestCreateGeneratesASecretKey checks that every room gets a secret key.
func TestCreateGeneratesASecretKey(t *testing.T) {
	reg := NewRegistry(10, 10)

	r, err := reg.Create()
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if r.Key() == "" {
		t.Fatal("the room has no secret key")
	}
	if len(r.Key()) != KeyLength {
		t.Fatalf("key length = %d, want %d", len(r.Key()), KeyLength)
	}
}

// TestSecretKeysAreUnique checks that two rooms never share a key.
func TestSecretKeysAreUnique(t *testing.T) {
	reg := NewRegistry(10, 10)

	seen := make(map[string]bool)
	for i := 0; i < MaxRooms; i++ {
		r, err := reg.Create()
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if seen[r.Key()] {
			t.Fatalf("the key %q was generated twice", r.Key())
		}
		seen[r.Key()] = true
	}
}

// TestMatchesKey checks the comparison of the secret key.
func TestMatchesKey(t *testing.T) {
	reg := NewRegistry(10, 10)

	r, err := reg.Create()
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if !r.MatchesKey(r.Key()) {
		t.Fatal("the room must match its own key")
	}
	if r.MatchesKey("wrong-key") {
		t.Fatal("the room must not match a wrong key")
	}
	if r.MatchesKey("") {
		t.Fatal("the room must not match an empty key")
	}
}

func TestRegistryGetIgnoresAnUnknownCode(t *testing.T) {
	reg := NewRegistry(10, 10)

	if _, ok := reg.Get("000000"); ok {
		t.Fatal("Get found a room that does not exist")
	}
}

func TestRegistryDeleteRemovesTheRoom(t *testing.T) {
	reg := NewRegistry(10, 10)
	created, err := reg.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	reg.Delete(created.Code())

	if _, ok := reg.Get(created.Code()); ok {
		t.Fatal("the room survived the deletion")
	}
	if reg.Count() != 0 {
		t.Fatalf("Count = %d, want 0", reg.Count())
	}
}

func TestIsValidCode(t *testing.T) {
	if !IsValidCode("123456") {
		t.Error("123456 must be valid")
	}
	for _, invalid := range []string{"", "12345", "1234567", "12345a", "abcdef"} {
		if IsValidCode(invalid) {
			t.Errorf("%q must be invalid", invalid)
		}
	}
}

// TestRegistryRefusesMoreThanTenRooms covers the limit of simultaneous rooms.
func TestRegistryRefusesMoreThanTenRooms(t *testing.T) {
	reg := NewRegistry(10, 10)

	for i := 0; i < MaxRooms; i++ {
		if _, err := reg.Create(); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}
	if reg.Count() != MaxRooms {
		t.Fatalf("Count = %d, want %d", reg.Count(), MaxRooms)
	}

	if _, err := reg.Create(); err == nil {
		t.Fatalf("Create accepted a room over the limit of %d", MaxRooms)
	}
	if reg.Count() != MaxRooms {
		t.Fatalf("Count = %d after a refused Create, want %d", reg.Count(), MaxRooms)
	}
}

// TestRegistryAllowsCreatingAgainAfterAFreeSlot covers that the limit frees
// up when a room is deleted.
func TestRegistryAllowsCreatingAgainAfterAFreeSlot(t *testing.T) {
	reg := NewRegistry(10, 10)

	var codes []string
	for i := 0; i < MaxRooms; i++ {
		r, err := reg.Create()
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		codes = append(codes, r.Code())
	}

	reg.Delete(codes[0])

	if _, err := reg.Create(); err != nil {
		t.Fatalf("Create after freeing a slot: %v", err)
	}
}
