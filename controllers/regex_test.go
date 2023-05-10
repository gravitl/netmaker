package controller

import "testing"

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
			Name: "nametoolong",
			Args: args{
				Name: "TestvalidNameTestvalidName",
			},
			Want: false,
		},
		{
			Name: "maxlength",
			Args: args{
				Name: "123456789012345",
			},
			Want: true,
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
