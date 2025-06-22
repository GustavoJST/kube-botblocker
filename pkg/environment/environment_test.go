package environment

import (
	"os"
	"reflect"
	"testing"
)

func TestGetOperatorEnv(t *testing.T) {
	origOperatorNS, operatorNSExists := os.LookupEnv("OPERATOR_NAMESPACE")
	origCurrentNSOnly, currentNSOnlyExists := os.LookupEnv("CURRENT_NAMESPACE_ONLY")

	t.Cleanup(func() {
		if operatorNSExists {
			os.Setenv("OPERATOR_NAMESPACE", origOperatorNS)
		} else {
			os.Unsetenv("OPERATOR_NAMESPACE")
		}

		if currentNSOnlyExists {
			os.Setenv("CURRENT_NAMESPACE_ONLY", origCurrentNSOnly)
		} else {
			os.Unsetenv("CURRENT_NAMESPACE_ONLY")
		}
	})

	tests := []struct {
		name    string
		env     map[string]string
		want    *OperatorEnv
		wantErr bool
	}{
		{
			name:    "No env",
			env:     map[string]string{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "No OPERATOR_NAMESPACE env",
			env: map[string]string{
				"CURRENT_NAMESPACE_ONLY": "false",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "No CURRENT_NAMESPACE_ONLY env",
			env: map[string]string{
				"OPERATOR_NAMESPACE": "default",
			},
			want: &OperatorEnv{
				OperatorNamespace:    "default",
				CurrentNamespaceOnly: false,
			},
			wantErr: false,
		},
		{
			name: "OPERATOR_NAMESPACE and CURRENT_NAMESPACE_ONLY set",
			env: map[string]string{
				"OPERATOR_NAMESPACE":     "default",
				"CURRENT_NAMESPACE_ONLY": "true",
			},
			want: &OperatorEnv{
				OperatorNamespace:    "default",
				CurrentNamespaceOnly: true,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("OPERATOR_NAMESPACE")
			os.Unsetenv("CURRENT_NAMESPACE_ONLY")

			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got, err := GetOperatorEnv()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetOperatorEnv() error - got: %v, expected: %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetOperatorEnv() value -  got: %+v, expected: %+v,", got, tt.want)
			}
		})
	}
}
