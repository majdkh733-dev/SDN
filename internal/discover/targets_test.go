package discover

import "testing"

func TestExpandBounds(t *testing.T) {
	hosts, err := Expand([]string{"10.0.0.1", "10.0.0.0/30"}, []string{"10.0.0.2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 3 {
		t.Fatalf("hosts %#v", hosts)
	}
	if _, err := Expand([]string{"10.0.0.0/23"}, nil); err == nil {
		t.Fatal("expected /23 to be rejected")
	}
	if _, err := Expand([]string{"10.1.0.0/24", "10.2.0.0/24"}, nil); err == nil {
		t.Fatal("expected combined range over 256 to be rejected")
	}
	if _, err := Expand(nil, nil); err == nil {
		t.Fatal("expected empty targets to be rejected")
	}
}
