package netmaker

import (
	"reflect"
	"testing"
)

var testDataset = &DNSEntries{
	"test1": hostAddresses{
		"skynet":  "10.0.0.1",
		"default": "100.64.0.3",
	},
	"test2": hostAddresses{
		"skynet":  "10.0.0.5",
		"default": "100.64.0.5",
	},
	"other-host": hostAddresses{
		"network": "192.168.1.1",
	},
}

func Test_getMatchingAEntries(t *testing.T) {
	type args struct {
		qn      string
		dataset *DNSEntries
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{

		{
			name: "simple request",
			args: args{
				qn:      "test1.skynet",
				dataset: testDataset,
			},
			want:    []string{"10.0.0.1"},
			wantErr: false,
		},
		{
			name: "simple request with trailing dot",
			args: args{
				qn:      "test1.skynet.",
				dataset: testDataset,
			},
			want:    []string{"10.0.0.1"},
			wantErr: false,
		},
		{
			name: "too long name shouldn't be valid",
			args: args{
				qn:      "test1.skynet.google.com",
				dataset: testDataset,
			},
			want:    []string{},
			wantErr: true,
		},
		{
			name: "too short name shouldn't be valid",
			args: args{
				qn:      "test1",
				dataset: testDataset,
			},
			want:    []string{},
			wantErr: true,
		},
		{
			name: "simple request on another network",
			args: args{
				qn:      "test2.default",
				dataset: testDataset,
			},
			want:    []string{"100.64.0.5"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getMatchingAEntries(tt.args.qn, tt.args.dataset)
			if (err != nil) != tt.wantErr {
				t.Errorf("getMatchingAEntries() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getMatchingAEntries() = %v, want %v", got, tt.want)
			}
		})
	}
}
