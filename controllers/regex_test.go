package controller

import "testing"

// TestValidName tests the validName function
func TestValidName(t *testing.T) {
	type args struct {
		Name string
	}
	tests := []struct {
		Name string
		Args args
		Want bool
	}{
		{
			Name: "validName",
			Args: args{
				Name: "TestvalidName",
			},
			Want: true,
		},
		{
			Name: "invalidName",
			Args: args{
				Name: "Test*Name",
			},
			Want: false,
		},
		{
			Name: "longname",
			Args: args{
				Name: "TestvalidNameTestvalidName",
			},
			Want: true,
		},
		{
			Name: "max length",
			Args: args{
				Name: "123456789012345",
			},
			Want: true,
		},
		{
			Name: "min length",
			Args: args{
				Name: "ama",
			},
			Want: false,
		},
		{
			Name: "toolong",
			Args: args{
				Name: "123456789012345123123123123123123123123123123",
			},
			Want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if got := validName(tt.Args.Name); got != tt.Want {
				t.Errorf("validName() = %v, want %v", got, tt.Want)
			}
		})
	}
}
