package logic

import "testing"

func Test_genKeyName(t *testing.T) {
	for i := 0; i < 100; i++ {
		kname := genKeyName()
		t.Log(kname)
		if len(kname) != 20 {
			t.Fatalf("improper length of key name, expected 20 got :%d", len(kname))
		}
	}
}

func Test_genKey(t *testing.T) {
	for i := 0; i < 100; i++ {
		kname := GenKey()
		t.Log(kname)
		if len(kname) != 16 {
			t.Fatalf("improper length of key name, expected 16 got :%d", len(kname))
		}
	}
}
