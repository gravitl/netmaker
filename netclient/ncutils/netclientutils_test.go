package ncutils

import "testing"

func TestMakeRandomString(t *testing.T) {
        for testCase := 0; testCase < 100; testCase++ {
                for size := 2; size < 2058; size++ {
                        if length := len(MakeRandomString(size)); length != size {
                                t.Fatalf("expected random string of size %d, got %d instead", size, length)
                        }
                }
        }
}


