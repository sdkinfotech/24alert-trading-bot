package order

import (
	"testing"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"
)

// TestMapExecutionStatus verifies every branch of the status mapping function.
func TestMapExecutionStatus_AllBranches(t *testing.T) {
	cases := []struct {
		input pb.OrderExecutionReportStatus
		want  OrderStatus
	}{
		{pb.OrderExecutionReportStatus_EXECUTION_REPORT_STATUS_NEW, OrderStatusNew},
		{pb.OrderExecutionReportStatus_EXECUTION_REPORT_STATUS_PARTIALLYFILL, OrderStatusPartiallyFilled},
		{pb.OrderExecutionReportStatus_EXECUTION_REPORT_STATUS_FILL, OrderStatusFilled},
		{pb.OrderExecutionReportStatus_EXECUTION_REPORT_STATUS_CANCELLED, OrderStatusCancelled},
		{pb.OrderExecutionReportStatus_EXECUTION_REPORT_STATUS_REJECTED, OrderStatusRejected},
		{pb.OrderExecutionReportStatus_EXECUTION_REPORT_STATUS_UNSPECIFIED, OrderStatusNew}, // default
	}

	for _, tc := range cases {
		got := MapExecutionStatus(tc.input)
		if got != tc.want {
			t.Errorf("mapExecutionStatus(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestNewService verifies constructor initialises all fields.
func TestNewService_Constructor(t *testing.T) {
	svc := NewService(nil, nil, NewRepository(), nil)
	if svc == nil {
		t.Fatal("NewService() returned nil")
	}
	if svc.idGen == nil {
		t.Error("idGen should be initialised by NewService")
	}
	if svc.repo == nil {
		t.Error("repo should be set by NewService")
	}
}

// TestOrderStatusConstants verifies that the string values are stable.
func TestOrderStatusConstants(t *testing.T) {
	cases := map[OrderStatus]string{
		OrderStatusNew:             "NEW",
		OrderStatusPartiallyFilled: "PARTIALLY_FILLED",
		OrderStatusFilled:          "FILLED",
		OrderStatusCancelled:       "CANCELLED",
		OrderStatusRejected:        "REJECTED",
		OrderStatusReplaced:        "REPLACED",
	}
	for status, want := range cases {
		if string(status) != want {
			t.Errorf("OrderStatus constant = %q, want %q", status, want)
		}
	}
}
