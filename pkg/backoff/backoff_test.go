package backoff

import (
	"testing"
	"time"
)

func TestExponential_Backoff(t *testing.T) {
	var testConfig = &Config{
		BaseDelay:  2.0 * time.Second,
		Multiplier: 2,
		Jitter:     0.2,
		MaxDelay:   60 * time.Second,
	}
	type fields struct {
		Config *Config
	}
	type args struct {
		retries int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "retry 0",
			fields: fields{
				Config: testConfig,
			},
			args: args{
				retries: 0,
			},
		},
		{
			name: "retry 1",
			fields: fields{
				Config: testConfig,
			},
			args: args{
				retries: 1,
			},
		},
		{
			name: "retry 2",
			fields: fields{
				Config: testConfig,
			},
			args: args{
				retries: 2,
			},
		},
		{
			name: "retry 3",
			fields: fields{
				Config: testConfig,
			},
			args: args{
				retries: 3,
			},
		},
		{
			name: "retry 4",
			fields: fields{
				Config: testConfig,
			},
			args: args{
				retries: 4,
			},
		},
		{
			name: "retry 5",
			fields: fields{
				Config: testConfig,
			},
			args: args{
				retries: 5,
			},
		},
		{
			name: "retry 10",
			fields: fields{
				Config: testConfig,
			},
			args: args{
				retries: 10,
			},
		},
		{
			name: "retry 20",
			fields: fields{
				Config: testConfig,
			},
			args: args{
				retries: 20,
			},
		},
		{
			name: "retry 40",
			fields: fields{
				Config: testConfig,
			},
			args: args{
				retries: 40,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := Exponential{
				Config: tt.fields.Config,
			}
			got := bc.Backoff(tt.args.retries)
			min, max := bc.Range(tt.args.retries)
			t.Logf("retry:%d min:%dms, max:%dms, backoff:%dms", tt.args.retries, min/time.Millisecond, max/time.Millisecond, got/time.Millisecond)
			if got < min || got > max {
				t.Error("backoff out range")
			}
		})
	}
}
