package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

const serviceName = "online-ordering-service"

type JSONLogger struct {
	service string
}

func (l *JSONLogger) log(level string, msg string, fields map[string]interface{}) {
	entry := map[string]interface{}{
		"ts":      time.Now().Format(time.RFC3339Nano),
		"level":   level,
		"service": l.service,
		"msg":     msg,
	}
	for k, v := range fields {
		entry[k] = v
	}
	json.NewEncoder(os.Stdout).Encode(entry)
}

func (l *JSONLogger) InfoContext(_ context.Context, msg string, kv ...interface{}) {
	l.log("INFO", msg, kvToMap(kv))
}

func (l *JSONLogger) DebugContext(_ context.Context, msg string, kv ...interface{}) {
	l.log("DEBUG", msg, kvToMap(kv))
}

func (l *JSONLogger) ErrorContext(_ context.Context, msg string, kv ...interface{}) {
	l.log("ERROR", msg, kvToMap(kv))
}

func kvToMap(kv []interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	for i := 0; i < len(kv)-1; i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			key = fmt.Sprintf("key%d", i/2)
		}
		m[key] = kv[i+1]
	}
	return m
}

var (
	tracer       = otel.Tracer(serviceName)
	meter        metric.Meter
	orderCounter metric.Int64Counter
	logger       = &JSONLogger{service: serviceName}
)

func initTracer() func() {
	ctx := context.Background()

	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to create OTLP trace exporter", "error", err)
		panic(err)
	}

	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to create OTLP metric exporter", "error", err)
		panic(err)
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)

	meter = otel.Meter(serviceName)
	orderCounter, err = meter.Int64Counter("orders_processed")
	if err != nil {
		logger.ErrorContext(ctx, "Failed to create counter", "error", err)
	}

	logger.InfoContext(ctx, "Tracer and Meter initialized", "service", serviceName)

	return func() {
		if err := tp.Shutdown(ctx); err != nil {
			logger.ErrorContext(ctx, "Error shutting down tracer", "error", err)
		} else {
			logger.InfoContext(ctx, "Tracer shutdown completed")
		}
	}
}

func main() {
	shutdownTracer := initTracer()
	defer shutdownTracer()

	ctx := context.Background()
	logger.InfoContext(ctx, "Service starting main loop")

	for {
		ctx, span := tracer.Start(ctx, "Ordering app")
		logger.DebugContext(ctx, "Ordering App Started")

		processorder(ctx)

		logger.DebugContext(ctx, "Ordering App Completed")
		span.End()

		time.Sleep(5 * time.Second)
	}
}

func processorder(ctx context.Context) {
	ctx, span := tracer.Start(ctx, "Process Order")
	defer span.End()

	logger.InfoContext(ctx, "Starting work", "task", "processing")
	span.SetAttributes(attribute.String("task", "processing"))
	span.AddEvent("Started Order processing")

	// Dummy metric increment
	orderCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("order.status", "completed")))

	step1(ctx)
	step2(ctx)
	step3(ctx)
	step4(ctx)
	step5(ctx)
	step6(ctx)
	step7(ctx)
	step8(ctx)
	step9(ctx)

	span.AddEvent("Finished Order processing")
	logger.InfoContext(ctx, "Order completed")
}

func step1(ctx context.Context) {
	_, span := tracer.Start(ctx, "Browse & Select Items")
	defer span.End()
	logger.DebugContext(ctx, "Executing Browse & Select Items", "detail", "Browse & Select Items")
	span.SetAttributes(attribute.String("step.detail", "Browse & Select Items"))
	time.Sleep(200 * time.Millisecond)
}

func step2(ctx context.Context) {
	_, span := tracer.Start(ctx, "Add to Cart")
	defer span.End()
	logger.DebugContext(ctx, "Executing Add to Cart", "detail", "Add to Cart")
	span.SetAttributes(attribute.String("step.detail", "Add to Cart"))
	time.Sleep(110 * time.Millisecond)
}

func step3(ctx context.Context) {
	_, span := tracer.Start(ctx, "Review Cart")
	defer span.End()
	logger.DebugContext(ctx, "Executing Review Cart", "detail", "Review Cart")
	span.SetAttributes(attribute.String("step.detail", "Review Cart"))
	time.Sleep(80 * time.Millisecond)
}

func step4(ctx context.Context) {
	_, span := tracer.Start(ctx, "Choose Delivery or Pickup")
	defer span.End()
	logger.DebugContext(
		ctx,
		"Executing Choose Delivery or Pickup",
		"detail",
		"Choose Delivery or Pickup",
	)
	span.SetAttributes(attribute.String("step.detail", "Choose Delivery or Pickup"))
	time.Sleep(90 * time.Millisecond)
}

func step5(ctx context.Context) {
	_, span := tracer.Start(ctx, "Enter Delivery Details")
	defer span.End()
	logger.DebugContext(ctx, "Executing Enter Delivery Details", "detail", "Enter Delivery Details")
	span.SetAttributes(attribute.String("step.detail", "Enter Delivery Details"))
	time.Sleep(50 * time.Millisecond)
}

func step6(ctx context.Context) {
	_, span := tracer.Start(ctx, "Select Payment Method")
	defer span.End()
	logger.DebugContext(ctx, "Executing Select Payment Method", "detail", "Select Payment Method")
	span.SetAttributes(attribute.String("step.detail", "Select Payment Method"))
	time.Sleep(100 * time.Millisecond)
}

func step7(ctx context.Context) {
	_, span := tracer.Start(ctx, "Review Order Summary")
	defer span.End()
	logger.DebugContext(ctx, "Executing Review Order Summary", "detail", "Review Order Summary")
	span.SetAttributes(attribute.String("step.detail", "Review Order Summary"))
	time.Sleep(40 * time.Millisecond)
}

func step8(ctx context.Context) {
	_, span := tracer.Start(ctx, "Place Order")
	defer span.End()
	logger.DebugContext(ctx, "Executing Place Order", "detail", "Place Order")
	span.SetAttributes(attribute.String("step.detail", "Place Order"))
	time.Sleep(30 * time.Millisecond)
}

func step9(ctx context.Context) {
	_, span := tracer.Start(ctx, "Receive Notifications")
	defer span.End()
	logger.DebugContext(ctx, "Executing Receive Notifications", "detail", "Receive Notifications")
	span.SetAttributes(attribute.String("step.detail", "Receive Notifications"))
	time.Sleep(10 * time.Millisecond)
}
