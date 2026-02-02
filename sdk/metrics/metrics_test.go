package metrics

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/glimps-re/go-gdetect/pkg/gdetect"
	gdetectmock "github.com/glimps-re/go-gdetect/pkg/gdetect/mock"
)

// newTestMetricsCollector creates a MetricsCollector initialized with the given metrics values.
func newTestMetricsCollector(cm ConnectorMetrics) (m *MetricsCollector) {
	m = &MetricsCollector{}
	m.itemsProcessed.Store(cm.ItemsProcessed)
	m.sizeProcessed.Store(cm.SizeProcessed)
	m.mitigatedItems.Store(cm.MitigatedItems)
	m.errorItems.Store(cm.ErrorItems)
	m.dailyQuota.Store(cm.DailyQuota)
	m.availableDailyQuota.Store(cm.AvailableDailyQuota)
	m.runningSince.Store(cm.RunningSince)
	return
}

func Test_MetricsCollector_GetAndStoreQuotas(t *testing.T) {
	mockErr := errors.New("mock error (test)")

	type fields struct {
		getProfileStatusError bool
		zeroQuotas            bool
	}
	type want struct {
		dailyQuota          int64
		availableDailyQuota int64
	}
	tests := []struct {
		name            string
		fields          fields
		want            want
		wantErr         bool
		wantSpecificErr error
	}{
		{
			name:            "ko GetProfileStatus error",
			fields:          fields{getProfileStatusError: true},
			wantErr:         true,
			wantSpecificErr: mockErr,
		},
		{
			name: "ok quotas stored",
			want: want{
				dailyQuota:          100,
				availableDailyQuota: 50,
			},
		},
		{
			name: "ok zero quotas",
			fields: fields{
				zeroQuotas: true,
			},
			want: want{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &gdetectmock.MockGDetectSubmitter{
				GetProfileStatusMock: func(ctx context.Context) (status gdetect.ProfileStatus, err error) {
					if tt.fields.getProfileStatusError {
						err = mockErr
						return
					}
					if !tt.fields.zeroQuotas {
						status = gdetect.ProfileStatus{
							DailyQuota:          100,
							AvailableDailyQuota: 50,
						}
					}
					return
				},
			}

			m := &MetricsCollector{
				detectClient: mock,
			}

			err := m.GetAndStoreQuotas(t.Context())

			if (err != nil) != tt.wantErr {
				t.Errorf("GetAndStoreQuotas() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantSpecificErr != nil && !errors.Is(err, tt.wantSpecificErr) {
				t.Errorf("GetAndStoreQuotas() want specific error %v, got %v", tt.wantSpecificErr, err)
				return
			}

			if err != nil {
				return
			}

			if got := m.dailyQuota.Load(); got != tt.want.dailyQuota {
				t.Errorf("dailyQuota = %v, want %v", got, tt.want.dailyQuota)
			}
			if got := m.availableDailyQuota.Load(); got != tt.want.availableDailyQuota {
				t.Errorf("availableDailyQuota = %v, want %v", got, tt.want.availableDailyQuota)
			}
		})
	}
}

func Test_MetricsCollector_GetAndReset(t *testing.T) {
	type fields struct {
		initialMetrics ConnectorMetrics
	}
	tests := []struct {
		name   string
		fields fields
		want   ConnectorMetrics
	}{
		{
			name: "ok empty metrics",
			want: ConnectorMetrics{},
		},
		{
			name: "ok only gauges set",
			fields: fields{
				initialMetrics: ConnectorMetrics{
					DailyQuota:          200,
					AvailableDailyQuota: 150,
					RunningSince:        5000,
				},
			},
			want: ConnectorMetrics{
				DailyQuota:          200,
				AvailableDailyQuota: 150,
				RunningSince:        5000,
			},
		},
		{
			name: "ok counters reset after read",
			fields: fields{
				initialMetrics: ConnectorMetrics{
					ItemsProcessed:      3,
					SizeProcessed:       1500,
					MitigatedItems:      2,
					ErrorItems:          1,
					DailyQuota:          100,
					AvailableDailyQuota: 50,
					RunningSince:        1000,
				},
			},
			want: ConnectorMetrics{
				ItemsProcessed:      3,
				SizeProcessed:       1500,
				MitigatedItems:      2,
				ErrorItems:          1,
				DailyQuota:          100,
				AvailableDailyQuota: 50,
				RunningSince:        1000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMetricsCollector(tt.fields.initialMetrics)

			got := m.GetAndReset()

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GetAndReset() mismatch (-want +got):\n%s", diff)
			}

			// check counters are reset to zero
			if m.itemsProcessed.Load() != 0 {
				t.Errorf("itemsProcessed not reset, got %v", m.itemsProcessed.Load())
			}
			if m.sizeProcessed.Load() != 0 {
				t.Errorf("sizeProcessed not reset, got %v", m.sizeProcessed.Load())
			}
			if m.mitigatedItems.Load() != 0 {
				t.Errorf("mitigatedItems not reset, got %v", m.mitigatedItems.Load())
			}
			if m.errorItems.Load() != 0 {
				t.Errorf("errorItems not reset, got %v", m.errorItems.Load())
			}

			// Verify gauges were NOT reset
			if m.dailyQuota.Load() != tt.fields.initialMetrics.DailyQuota {
				t.Errorf("dailyQuota should not be reset, got %v", m.dailyQuota.Load())
			}
			if m.availableDailyQuota.Load() != tt.fields.initialMetrics.AvailableDailyQuota {
				t.Errorf("availableDailyQuota should not be reset, got %v", m.availableDailyQuota.Load())
			}
			if m.runningSince.Load() != tt.fields.initialMetrics.RunningSince {
				t.Errorf("runningSince should not be reset, got %v", m.runningSince.Load())
			}
		})
	}
}

func Test_MetricsCollector_RestoreCounterMetrics(t *testing.T) {
	type fields struct {
		initialMetrics ConnectorMetrics
	}
	type args struct {
		metrics ConnectorMetrics
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   ConnectorMetrics
	}{
		{
			name: "ok",
			args: args{
				metrics: ConnectorMetrics{
					ItemsProcessed: 5,
					SizeProcessed:  2000,
					MitigatedItems: 3,
					ErrorItems:     1,
				},
			},
			want: ConnectorMetrics{
				ItemsProcessed: 5,
				SizeProcessed:  2000,
				MitigatedItems: 3,
				ErrorItems:     1,
			},
		},
		{
			name: "ok restore adds to existing counters",
			fields: fields{
				initialMetrics: ConnectorMetrics{
					ItemsProcessed: 2,
					SizeProcessed:  500,
					MitigatedItems: 1,
					ErrorItems:     1,
				},
			},
			args: args{
				metrics: ConnectorMetrics{
					ItemsProcessed: 5,
					SizeProcessed:  2000,
					MitigatedItems: 3,
					ErrorItems:     1,
				},
			},
			want: ConnectorMetrics{
				ItemsProcessed: 7,
				SizeProcessed:  2500,
				MitigatedItems: 4,
				ErrorItems:     2,
			},
		},
		{
			name: "ok restore zero metrics",
			fields: fields{
				initialMetrics: ConnectorMetrics{
					ItemsProcessed: 10,
					SizeProcessed:  3000,
					MitigatedItems: 5,
					ErrorItems:     2,
				},
			},
			args: args{
				metrics: ConnectorMetrics{},
			},
			want: ConnectorMetrics{
				ItemsProcessed: 10,
				SizeProcessed:  3000,
				MitigatedItems: 5,
				ErrorItems:     2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMetricsCollector(tt.fields.initialMetrics)

			m.RestoreCounterMetrics(tt.args.metrics)

			got := ConnectorMetrics{
				ItemsProcessed: m.itemsProcessed.Load(),
				SizeProcessed:  m.sizeProcessed.Load(),
				MitigatedItems: m.mitigatedItems.Load(),
				ErrorItems:     m.errorItems.Load(),
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("RestoreCounterMetrics() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_MetricsCollector_AddItemProcessed(t *testing.T) {
	type args struct {
		size int64
	}
	tests := []struct {
		name               string
		args               args
		wantItemsProcessed int64
		wantSizeProcessed  int64
	}{
		{
			name: "ok",
			args: args{
				size: 1024,
			},
			wantItemsProcessed: 1,
			wantSizeProcessed:  1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MetricsCollector{}

			m.AddItemProcessed(tt.args.size)

			if got := m.itemsProcessed.Load(); got != tt.wantItemsProcessed {
				t.Errorf("itemsProcessed = %v, want %v", got, tt.wantItemsProcessed)
			}
			if got := m.sizeProcessed.Load(); got != tt.wantSizeProcessed {
				t.Errorf("sizeProcessed = %v, want %v", got, tt.wantSizeProcessed)
			}
		})
	}
}

func Test_MetricsCollector_AddErrorItem(t *testing.T) {
	tests := []struct {
		name           string
		wantErrorItems int64
	}{
		{
			name:           "ok",
			wantErrorItems: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MetricsCollector{}

			m.AddErrorItem()

			if got := m.errorItems.Load(); got != tt.wantErrorItems {
				t.Errorf("errorItems = %v, want %v", got, tt.wantErrorItems)
			}
		})
	}
}

func Test_MetricsCollector_AddMitigatedItem(t *testing.T) {
	tests := []struct {
		name               string
		wantMitigatedItems int64
	}{
		{
			name:               "ok",
			wantMitigatedItems: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MetricsCollector{}

			m.AddMitigatedItem()

			if got := m.mitigatedItems.Load(); got != tt.wantMitigatedItems {
				t.Errorf("mitigatedItems = %v, want %v", got, tt.wantMitigatedItems)
			}
		})
	}
}
