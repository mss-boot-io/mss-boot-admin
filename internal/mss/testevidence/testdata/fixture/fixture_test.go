package fixture

import "testing"

func TestPass(t *testing.T) {}

func TestSkip(t *testing.T) {
	t.Skip("intentional exact-evidence fixture")
}

func TestFail(t *testing.T) {
	t.Fatal("intentional exact-evidence fixture")
}
