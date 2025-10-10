package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseValidatorPowers(t *testing.T) {
	tests := []struct {
		name          string
		powersStr     string
		numValidators int
		want          []int64
		wantErr       bool
	}{
		{
			name:          "empty string - use defaults",
			powersStr:     "",
			numValidators: 4,
			want:          []int64{100, 100, 100, 100},
			wantErr:       false,
		},
		{
			name:          "exact number of powers",
			powersStr:     "100,200,150,300",
			numValidators: 4,
			want:          []int64{100, 200, 150, 300},
			wantErr:       false,
		},
		{
			name:          "fewer powers than validators",
			powersStr:     "100,200",
			numValidators: 5,
			want:          []int64{100, 200, 200, 200, 200},
			wantErr:       false,
		},
		{
			name:          "single power for all validators",
			powersStr:     "500",
			numValidators: 3,
			want:          []int64{500, 500, 500},
			wantErr:       false,
		},
		{
			name:          "more powers than validators",
			powersStr:     "100,200,300,400",
			numValidators: 2,
			want:          []int64{100, 200},
			wantErr:       false,
		},
		{
			name:          "invalid power - not a number",
			powersStr:     "100,abc,300",
			numValidators: 3,
			want:          nil,
			wantErr:       true,
		},
		{
			name:          "invalid power - negative",
			powersStr:     "100,-200,300",
			numValidators: 3,
			want:          nil,
			wantErr:       true,
		},
		{
			name:          "invalid power - zero",
			powersStr:     "100,0,300",
			numValidators: 3,
			want:          nil,
			wantErr:       true,
		},
		{
			name:          "with spaces",
			powersStr:     "100, 200, 300",
			numValidators: 3,
			want:          []int64{100, 200, 300},
			wantErr:       false,
		},
		{
			name:          "trailing comma",
			powersStr:     "100,200,300,",
			numValidators: 3,
			want:          []int64{100, 200, 300},
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseValidatorPowers(tt.powersStr, tt.numValidators)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
		})
	}
}
