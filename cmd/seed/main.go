package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"time"

	"google.golang.org/protobuf/proto"

	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	metricscollectorpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

var models = []string{"claude-opus-4", "claude-sonnet-4", "claude-haiku-4-5", "gpt-4o", "o3"}
var agents = []string{"bosun", "claude-code", "codex", "aider"}
var tools = []string{"Bash", "Read", "Edit", "Write", "Grep", "Glob"}

func main() {
	endpoint := flag.String("endpoint", "http://localhost:4318", "OTLP HTTP endpoint")
	sessions := flag.Int("sessions", 5, "Number of agent sessions to generate")
	standaloneTraces := flag.Int("traces", 10, "Number of standalone traces")
	flag.Parse()

	log.Printf("Seeding to %s: %d sessions, %d standalone traces", *endpoint, *sessions, *standaloneTraces)

	// Generate agent sessions
	for i := 0; i < *sessions; i++ {
		sendSession(*endpoint, agents[i%len(agents)])
		time.Sleep(50 * time.Millisecond)
	}

	// Generate standalone traces (non-agent)
	for i := 0; i < *standaloneTraces; i++ {
		sendStandaloneTrace(*endpoint)
		time.Sleep(50 * time.Millisecond)
	}

	// Send some metrics
	sendMetrics(*endpoint)

	log.Println("Seeding complete!")
}

func sendSession(endpoint, agent string) {
	sessionID := randomHex(16)
	traceID := randomBytes(16)
	now := time.Now()

	var spans []*tracepb.Span
	model := models[randInt(len(models))]

	// Parent: agent.session span
	sessionSpanID := randomBytes(8)
	sessionStart := now.Add(-time.Duration(randInt(300)+60) * time.Second)
	sessionEnd := now.Add(-time.Duration(randInt(30)) * time.Second)
	spans = append(spans, &tracepb.Span{
		TraceId:           traceID,
		SpanId:            sessionSpanID,
		Name:              "agent.session",
		Kind:              tracepb.Span_SPAN_KIND_SERVER,
		StartTimeUnixNano: uint64(sessionStart.UnixNano()),
		EndTimeUnixNano:   uint64(sessionEnd.UnixNano()),
		Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		Attributes: []*commonpb.KeyValue{
			strAttr("agent.name", agent),
			strAttr("agent.session_id", sessionID),
			strAttr("agent.task_id", fmt.Sprintf("task-%s", randomHex(4))),
		},
	})

	// Child spans: chat completions + tool calls
	numSteps := randInt(5) + 3
	cursor := sessionStart.Add(time.Second)
	for j := 0; j < numSteps; j++ {
		// Chat completion
		chatSpanID := randomBytes(8)
		inputTokens := int64(randInt(10000) + 500)
		outputTokens := int64(randInt(5000) + 100)
		chatDur := time.Duration(randInt(30)+2) * time.Second
		chatStart := cursor
		chatEnd := chatStart.Add(chatDur)

		statusCode := tracepb.Status_STATUS_CODE_OK
		if randInt(20) == 0 { // 5% error rate
			statusCode = tracepb.Status_STATUS_CODE_ERROR
		}

		spans = append(spans, &tracepb.Span{
			TraceId:           traceID,
			SpanId:            chatSpanID,
			ParentSpanId:      sessionSpanID,
			Name:              "gen_ai.chat_completion",
			Kind:              tracepb.Span_SPAN_KIND_CLIENT,
			StartTimeUnixNano: uint64(chatStart.UnixNano()),
			EndTimeUnixNano:   uint64(chatEnd.UnixNano()),
			Status:            &tracepb.Status{Code: statusCode},
			Attributes: []*commonpb.KeyValue{
				strAttr("gen_ai.system", "anthropic"),
				strAttr("gen_ai.request.model", model),
				intAttr("gen_ai.usage.input_tokens", inputTokens),
				intAttr("gen_ai.usage.output_tokens", outputTokens),
				strAttr("agent.name", agent),
				strAttr("agent.session_id", sessionID),
			},
		})
		cursor = chatEnd.Add(500 * time.Millisecond)

		// Tool calls after chat
		numTools := randInt(3)
		for k := 0; k < numTools; k++ {
			tool := tools[randInt(len(tools))]
			toolSpanID := randomBytes(8)
			toolDur := time.Duration(randInt(5000)+100) * time.Millisecond
			toolStart := cursor
			toolEnd := toolStart.Add(toolDur)

			spans = append(spans, &tracepb.Span{
				TraceId:           traceID,
				SpanId:            toolSpanID,
				ParentSpanId:      chatSpanID,
				Name:              "gen_ai.tool_call",
				Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
				StartTimeUnixNano: uint64(toolStart.UnixNano()),
				EndTimeUnixNano:   uint64(toolEnd.UnixNano()),
				Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
				Attributes: []*commonpb.KeyValue{
					strAttr("gen_ai.tool.name", tool),
					strAttr("gen_ai.tool.call_id", fmt.Sprintf("call_%s", randomHex(6))),
					strAttr("agent.name", agent),
					strAttr("agent.session_id", sessionID),
				},
			})
			cursor = toolEnd.Add(100 * time.Millisecond)
		}
	}

	sendTraceSpans(endpoint, spans, agent)
	log.Printf("  session %s (%s): %d spans, model=%s", sessionID[:8], agent, len(spans), model)
}

func sendStandaloneTrace(endpoint string) {
	traceID := randomBytes(16)
	model := models[randInt(len(models))]
	now := time.Now()
	start := now.Add(-time.Duration(randInt(60)+5) * time.Second)

	inputTokens := int64(randInt(8000) + 200)
	outputTokens := int64(randInt(4000) + 50)
	dur := time.Duration(randInt(20)+1) * time.Second

	span := &tracepb.Span{
		TraceId:           traceID,
		SpanId:            randomBytes(8),
		Name:              "gen_ai.chat_completion",
		Kind:              tracepb.Span_SPAN_KIND_CLIENT,
		StartTimeUnixNano: uint64(start.UnixNano()),
		EndTimeUnixNano:   uint64(start.Add(dur).UnixNano()),
		Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		Attributes: []*commonpb.KeyValue{
			strAttr("gen_ai.system", "openai"),
			strAttr("gen_ai.request.model", model),
			intAttr("gen_ai.usage.input_tokens", inputTokens),
			intAttr("gen_ai.usage.output_tokens", outputTokens),
		},
	}

	sendTraceSpans(endpoint, []*tracepb.Span{span}, "")
}

func sendTraceSpans(endpoint string, spans []*tracepb.Span, agent string) {
	var resAttrs []*commonpb.KeyValue
	resAttrs = append(resAttrs, strAttr("service.name", "lookout-seed"))
	if agent != "" {
		resAttrs = append(resAttrs, strAttr("agent.name", agent))
	}

	req := &collectorpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{Attributes: resAttrs},
				ScopeSpans: []*tracepb.ScopeSpans{
					{Spans: spans},
				},
			},
		},
	}

	body, err := proto.Marshal(req)
	if err != nil {
		log.Printf("  ERROR marshal: %v", err)
		return
	}

	resp, err := http.Post(endpoint+"/v1/traces", "application/x-protobuf", bytes.NewReader(body))
	if err != nil {
		log.Printf("  ERROR send traces: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("  ERROR traces status: %d", resp.StatusCode)
	}
}

func sendMetrics(endpoint string) {
	now := time.Now()
	req := &metricscollectorpb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						strAttr("service.name", "lookout-seed"),
					},
				},
				ScopeMetrics: []*metricspb.ScopeMetrics{
					{
						Metrics: []*metricspb.Metric{
							{
								Name: "llm.request.duration",
								Data: &metricspb.Metric_Histogram{
									Histogram: &metricspb.Histogram{
										DataPoints: []*metricspb.HistogramDataPoint{
											{
												TimeUnixNano:   uint64(now.UnixNano()),
												Count:          42,
												Sum:            ptr(126.5),
												Min:            ptr(0.5),
												Max:            ptr(15.2),
												Attributes:     []*commonpb.KeyValue{strAttr("model", "claude-sonnet-4")},
											},
										},
									},
								},
							},
							{
								Name: "llm.token.usage",
								Data: &metricspb.Metric_Sum{
									Sum: &metricspb.Sum{
										DataPoints: []*metricspb.NumberDataPoint{
											{
												TimeUnixNano: uint64(now.UnixNano()),
												Value:        &metricspb.NumberDataPoint_AsInt{AsInt: 150000},
												Attributes:   []*commonpb.KeyValue{strAttr("type", "input"), strAttr("model", "claude-sonnet-4")},
											},
											{
												TimeUnixNano: uint64(now.UnixNano()),
												Value:        &metricspb.NumberDataPoint_AsInt{AsInt: 45000},
												Attributes:   []*commonpb.KeyValue{strAttr("type", "output"), strAttr("model", "claude-sonnet-4")},
											},
										},
									},
								},
							},
							{
								Name: "active_sessions",
								Data: &metricspb.Metric_Gauge{
									Gauge: &metricspb.Gauge{
										DataPoints: []*metricspb.NumberDataPoint{
											{
												TimeUnixNano: uint64(now.UnixNano()),
												Value:        &metricspb.NumberDataPoint_AsInt{AsInt: 3},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	body, err := proto.Marshal(req)
	if err != nil {
		log.Printf("  ERROR marshal metrics: %v", err)
		return
	}

	resp, err := http.Post(endpoint+"/v1/metrics", "application/x-protobuf", bytes.NewReader(body))
	if err != nil {
		log.Printf("  ERROR send metrics: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("  ERROR metrics status: %d", resp.StatusCode)
	} else {
		log.Println("  metrics sent successfully")
	}
}

func ptr(f float64) *float64 { return &f }

func strAttr(key, val string) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: val}},
	}
}

func intAttr(key string, val int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: val}},
	}
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

func randomHex(n int) string {
	return hex.EncodeToString(randomBytes(n))
}

func randInt(max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}
