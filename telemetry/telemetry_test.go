package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestStartLinkedRootSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { otel.SetTracerProvider(previous) })

	runCtx, run := StartSpan(context.Background(), "run")
	_, item := StartLinkedRootSpan(runCtx, "item", Attrs{"event_id": int64(42), "room": "101", "retries": 3, "ok": true, "rate": 1.5, "other": []int{1}})
	item.End()
	run.End()

	got := recorder.Ended()[0]
	runSC := run.SpanContext()
	if got.SpanContext().TraceID() == runSC.TraceID() {
		t.Fatal("item span shares the run's trace_id")
	}
	if got.Parent().IsValid() {
		t.Fatal("item span has a parent, want a new root")
	}
	if links := got.Links(); len(links) != 1 || links[0].SpanContext.SpanID() != runSC.SpanID() {
		t.Fatalf("links = %v, want one link to the run span", links)
	}

	want := map[attribute.Key]attribute.Value{
		"event_id": attribute.Int64Value(42),
		"room":     attribute.StringValue("101"),
		"retries":  attribute.IntValue(3),
		"ok":       attribute.BoolValue(true),
		"rate":     attribute.Float64Value(1.5),
		"other":    attribute.StringValue("[1]"),
	}
	attrs := got.Attributes()
	if len(attrs) != len(want) {
		t.Fatalf("got %d attributes, want %d", len(attrs), len(want))
	}
	for _, kv := range attrs {
		if want[kv.Key] != kv.Value {
			t.Errorf("%s = %v, want %v", kv.Key, kv.Value.Emit(), want[kv.Key].Emit())
		}
	}
}
