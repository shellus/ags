package selfupdate

import "testing"

func TestPlatformAssetName(t *testing.T) {
	name, err := platformAssetName("windows", "amd64")
	if err != nil || name != "ags-windows-amd64.exe" {
		t.Fatalf("platformAssetName() = %q, %v", name, err)
	}
}

func TestFindChecksum(t *testing.T) {
	value := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  ags-linux-amd64\n"
	checksum, err := findChecksum([]byte(value), "ags-linux-amd64")
	if err != nil || checksum != value[:64] {
		t.Fatalf("findChecksum() = %q, %v", checksum, err)
	}
}
