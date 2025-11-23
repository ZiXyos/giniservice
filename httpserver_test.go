package httpservice_test

import "testing"

func TestCreateNewServer(t *testing.T) {
	t.Parallel()
	type args struct{}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "CreateNewServerFromDefault",
			args: args{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
		})
	}
}
