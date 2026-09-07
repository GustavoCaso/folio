package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ardanlabs/kronk/sdk/kronk/model"
)

func TestAIConfigFromEnv_Defaults(t *testing.T) {
	cfg := AIConfigFromEnv(func(string) string { return "" })

	if cfg.Model == "" {
		t.Error("expected non-empty default model")
	}
	if cfg.TimeoutBase != 120*time.Second {
		t.Errorf("unexpected default TimeoutBase: %v", cfg.TimeoutBase)
	}
	if cfg.TimeoutPerBlock != 5*time.Second {
		t.Errorf("unexpected default TimeoutPerBlock: %v", cfg.TimeoutPerBlock)
	}
	if cfg.KronkLibPath != "" {
		t.Errorf("expected empty KronkLibPath, got %q", cfg.KronkLibPath)
	}
}

func TestAIConfigFromEnv_Overrides(t *testing.T) {
	env := map[string]string{
		"AI_PIPELINE_MODEL":             "some/other-model",
		"AI_PIPELINE_TIMEOUT_BASE":      "30s",
		"AI_PIPELINE_TIMEOUT_PER_BLOCK": "2s",
		"AI_PIPELINE_KRONK_LIB_PATH":    "/tmp/libs",
	}
	cfg := AIConfigFromEnv(func(k string) string { return env[k] })

	if cfg.Model != "some/other-model" {
		t.Errorf("unexpected model: %q", cfg.Model)
	}
	if cfg.TimeoutBase != 30*time.Second {
		t.Errorf("unexpected TimeoutBase: %v", cfg.TimeoutBase)
	}
	if cfg.TimeoutPerBlock != 2*time.Second {
		t.Errorf("unexpected TimeoutPerBlock: %v", cfg.TimeoutPerBlock)
	}
	if cfg.KronkLibPath != "/tmp/libs" {
		t.Errorf("unexpected KronkLibPath: %q", cfg.KronkLibPath)
	}
}

func TestAIConfigFromEnv_InvalidDurationFallsBackToDefault(t *testing.T) {
	env := map[string]string{"AI_PIPELINE_TIMEOUT_BASE": "not-a-duration"}
	cfg := AIConfigFromEnv(func(k string) string { return env[k] })

	if cfg.TimeoutBase != 120*time.Second {
		t.Errorf("expected default TimeoutBase on invalid input, got %v", cfg.TimeoutBase)
	}
}

// fakeChatter is a kronkChatter test double returning a fixed JSON response.
type fakeChatter struct {
	content string
	err     error
}

func (f *fakeChatter) ChatStreaming(ctx context.Context, d model.D) (<-chan model.ChatResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	ch := make(chan model.ChatResponse, 1)
	ch <- model.ChatResponse{
		Choices: []model.Choice{{
			Delta: &model.ResponseMessage{Content: f.content},
		}},
	}
	close(ch)
	return ch, nil
}

// finalDeltaBundlesContentChatter is a kronkChatter test double that
// splits its JSON response across two streamed deltas, with the second
// (terminal) delta carrying both trailing content and FinishReasonStop --
// reproducing a real streaming shape where the stop signal and the last
// bit of content arrive in the same chunk.
type finalDeltaBundlesContentChatter struct {
	firstHalf, secondHalf string
}

func (f finalDeltaBundlesContentChatter) ChatStreaming(ctx context.Context, d model.D) (<-chan model.ChatResponse, error) {
	stopReason := model.FinishReasonStop
	ch := make(chan model.ChatResponse, 2)
	ch <- model.ChatResponse{Choices: []model.Choice{{
		Delta: &model.ResponseMessage{Content: f.firstHalf},
	}}}
	ch <- model.ChatResponse{Choices: []model.Choice{{
		Delta:           &model.ResponseMessage{Content: f.secondHalf},
		FinishReasonPtr: &stopReason,
	}}}
	close(ch)
	return ch, nil
}

func TestAICleaner_Clean_FinalStopDeltaContentIsNotDropped(t *testing.T) {
	resp := aiSegmentResult{Blocks: []aiSegmentedBlock{{Kind: "prose", Text: "hello"}}}
	raw, _ := json.Marshal(resp)
	// Split the JSON partway through so the terminal delta (carrying
	// FinishReasonStop) has to contribute real content for Unmarshal to
	// succeed -- if that content were dropped, this would fail to parse.
	split := len(raw) / 2

	c := &aiCleaner{
		cfg: AIConfig{TimeoutBase: time.Second},
		client: finalDeltaBundlesContentChatter{
			firstHalf:  string(raw[:split]),
			secondHalf: string(raw[split:]),
		},
	}
	c.initOnce.Do(func() {})

	got, err := c.Clean(context.Background(), []textRect{{Text: "x"}})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if len(got) != 1 || got[0].Text != "hello" {
		t.Errorf("expected the terminal delta's content to be included, got %+v", got)
	}
}

func TestAICleaner_Clean_EmptyRectsSkipsCall(t *testing.T) {
	c := &aiCleaner{cfg: AIConfig{TimeoutBase: time.Second}, client: &fakeChatter{err: errors.New("should not be called")}}
	got, err := c.Clean(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil result for empty input, got %+v", got)
	}
}

func TestAICleaner_Clean_ParsesModelResponse(t *testing.T) {
	resp := aiSegmentResult{Blocks: []aiSegmentedBlock{
		{Kind: "h1", Text: "Chapter 1"},
		{Kind: "prose", Text: "Some intro text."},
		{Kind: "code", Text: "fn main() {}"},
	}}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}

	c := &aiCleaner{
		cfg:    AIConfig{TimeoutBase: time.Second},
		client: &fakeChatter{content: string(raw)},
	}
	// Skip ensureLoaded by marking initOnce already done (no real model load).
	c.initOnce.Do(func() {})

	got, err := c.Clean(context.Background(), []textRect{
		{Text: "Chapter 1", FontName: "FuturaStd-CondensedBold"},
		{Text: "Some intro text.", FontName: "Georgia"},
		{Text: "fn main() {}", FontName: "Courier"},
	})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 blocks, got %d: %+v", len(got), got)
	}
	if got[0].Kind != blockH1 || got[0].Text != "Chapter 1" {
		t.Errorf("block 0 unexpected: %+v", got[0])
	}
	if got[1].Kind != blockProse || got[1].Text != "Some intro text." {
		t.Errorf("block 1 unexpected: %+v", got[1])
	}
	if got[2].Kind != blockCode || got[2].Text != "fn main() {}" {
		t.Errorf("block 2 unexpected: %+v", got[2])
	}
}

func TestAICleaner_Clean_UnknownKindDefaultsToProse(t *testing.T) {
	resp := aiSegmentResult{Blocks: []aiSegmentedBlock{{Kind: "bogus", Text: "text"}}}
	raw, _ := json.Marshal(resp)

	c := &aiCleaner{cfg: AIConfig{TimeoutBase: time.Second}, client: &fakeChatter{content: string(raw)}}
	c.initOnce.Do(func() {})

	got, err := c.Clean(context.Background(), []textRect{{Text: "text"}})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if len(got) != 1 || got[0].Kind != blockProse {
		t.Errorf("expected unknown kind to default to prose, got %+v", got)
	}
}

func TestAICleaner_Clean_ChatErrorSurfacedToCaller(t *testing.T) {
	c := &aiCleaner{
		cfg:    AIConfig{TimeoutBase: time.Second},
		client: &fakeChatter{err: errors.New("boom")},
	}
	c.initOnce.Do(func() {})

	_, err := c.Clean(context.Background(), []textRect{{Text: "x"}})
	if err == nil {
		t.Fatal("expected error from Clean when chat streaming fails")
	}
}

func TestAICleaner_Clean_MalformedJSONReturnsError(t *testing.T) {
	c := &aiCleaner{
		cfg:    AIConfig{TimeoutBase: time.Second},
		client: &fakeChatter{content: "not json"},
	}
	c.initOnce.Do(func() {})

	_, err := c.Clean(context.Background(), []textRect{{Text: "x"}})
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
}

// sequencedChatter is a kronkChatter test double that returns one block per
// call, in order, recording how many rects each call's prompt referenced
// (by counting "top:" occurrences in the request text) so tests can assert
// on chunk sizes without depending on Clean's internal chunking constant.
type sequencedChatter struct {
	calls     int
	rectsSeen []int
}

func (f *sequencedChatter) ChatStreaming(ctx context.Context, d model.D) (<-chan model.ChatResponse, error) {
	f.calls++

	messages, _ := d["messages"].([]model.D)
	var promptText string
	for _, m := range messages {
		if content, ok := m["content"].(string); ok {
			promptText += content
		}
	}
	f.rectsSeen = append(f.rectsSeen, countOccurrences(promptText, "top:"))

	resp := aiSegmentResult{Blocks: []aiSegmentedBlock{
		{Kind: "prose", Text: fmt.Sprintf("chunk-%d", f.calls)},
	}}
	raw, _ := json.Marshal(resp)

	ch := make(chan model.ChatResponse, 1)
	ch <- model.ChatResponse{Choices: []model.Choice{{Delta: &model.ResponseMessage{Content: string(raw)}}}}
	close(ch)
	return ch, nil
}

func countOccurrences(s, substr string) int {
	n := 0
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			n++
			i += len(substr) - 1
		}
	}
	return n
}

func TestAICleaner_Clean_ChunksLargeRectListsAcrossMultipleCalls(t *testing.T) {
	rects := make([]textRect, chunkSize+5)
	for i := range rects {
		rects[i] = textRect{Text: fmt.Sprintf("line-%d", i)}
	}

	fake := &sequencedChatter{}
	c := &aiCleaner{cfg: AIConfig{TimeoutBase: time.Second}, client: fake}
	c.initOnce.Do(func() {})

	got, err := c.Clean(context.Background(), rects)
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}

	if fake.calls != 2 {
		t.Fatalf("expected 2 chunked calls for %d rects, got %d", len(rects), fake.calls)
	}
	if fake.rectsSeen[0] != chunkSize {
		t.Errorf("expected first chunk to carry %d rects, got %d", chunkSize, fake.rectsSeen[0])
	}
	if fake.rectsSeen[1] != 5 {
		t.Errorf("expected second chunk to carry 5 rects, got %d", fake.rectsSeen[1])
	}

	if len(got) != 2 {
		t.Fatalf("expected one block per chunk (2 total), got %d: %+v", len(got), got)
	}
	if got[0].Text != "chunk-1" || got[1].Text != "chunk-2" {
		t.Errorf("expected chunk blocks in call order, got %+v", got)
	}
}

func TestAICleaner_Clean_SingleChunkMakesOneCall(t *testing.T) {
	fake := &sequencedChatter{}
	c := &aiCleaner{cfg: AIConfig{TimeoutBase: time.Second}, client: fake}
	c.initOnce.Do(func() {})

	rects := []textRect{{Text: "a"}, {Text: "b"}}
	got, err := c.Clean(context.Background(), rects)
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if fake.calls != 1 {
		t.Errorf("expected 1 call for a rect list under chunkSize, got %d", fake.calls)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 block, got %d: %+v", len(got), got)
	}
}

func TestAICleaner_Clean_ChunkErrorSurfacedToCaller(t *testing.T) {
	rects := make([]textRect, chunkSize+1)
	for i := range rects {
		rects[i] = textRect{Text: "x"}
	}

	c := &aiCleaner{
		cfg:    AIConfig{TimeoutBase: time.Second},
		client: &fakeChatter{err: errors.New("boom")},
	}
	c.initOnce.Do(func() {})

	_, err := c.Clean(context.Background(), rects)
	if err == nil {
		t.Fatal("expected error when a chunk's chat call fails")
	}
}

// truncatingThenSucceedingChatter is a kronkChatter test double simulating
// a degenerate-repetition truncation (FinishReasonLength) on its first
// call and a valid response on the second, recording the temperature each
// call requested so tests can assert cleanChunk's retry bumps it.
type truncatingThenSucceedingChatter struct {
	calls        int
	temperatures []float64
}

func (f *truncatingThenSucceedingChatter) ChatStreaming(ctx context.Context, d model.D) (<-chan model.ChatResponse, error) {
	f.calls++
	temp, _ := d["temperature"].(float64)
	f.temperatures = append(f.temperatures, temp)

	ch := make(chan model.ChatResponse, 1)
	if f.calls == 1 {
		lengthReason := model.FinishReasonLength
		ch <- model.ChatResponse{Choices: []model.Choice{{
			Delta:           &model.ResponseMessage{Content: `{"blocks":[{"kind":"prose","text":"wasm-`},
			FinishReasonPtr: &lengthReason,
		}}}
		close(ch)
		return ch, nil
	}

	resp := aiSegmentResult{Blocks: []aiSegmentedBlock{{Kind: "prose", Text: "recovered"}}}
	raw, _ := json.Marshal(resp)
	ch <- model.ChatResponse{Choices: []model.Choice{{Delta: &model.ResponseMessage{Content: string(raw)}}}}
	close(ch)
	return ch, nil
}

func TestAICleaner_CleanChunk_RetriesOnceWithHigherTemperatureAfterTruncation(t *testing.T) {
	fake := &truncatingThenSucceedingChatter{}
	c := &aiCleaner{
		cfg:    AIConfig{TimeoutBase: time.Second, Temperature: 0.2, RetryTemperatureBump: 0.4},
		client: fake,
	}
	c.initOnce.Do(func() {})

	got, err := c.Clean(context.Background(), []textRect{{Text: "x"}})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if fake.calls != 2 {
		t.Fatalf("expected 2 calls (initial + 1 retry), got %d", fake.calls)
	}
	if fake.temperatures[0] != 0.2 {
		t.Errorf("expected first call at configured temperature 0.2, got %v", fake.temperatures[0])
	}
	wantRetryTemp := 0.6
	if diff := fake.temperatures[1] - wantRetryTemp; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("expected retry temperature %v, got %v", wantRetryTemp, fake.temperatures[1])
	}
	if len(got) != 1 || got[0].Text != "recovered" {
		t.Errorf("expected the retry's successful result to be used, got %+v", got)
	}
}

func TestAICleaner_CleanChunk_FailsAfterRetryAlsoTruncates(t *testing.T) {
	c := &aiCleaner{
		cfg:    AIConfig{TimeoutBase: time.Second, Temperature: 0.2},
		client: alwaysTruncatingChatter{},
	}
	c.initOnce.Do(func() {})

	_, err := c.Clean(context.Background(), []textRect{{Text: "x"}})
	if err == nil {
		t.Fatal("expected error when both the initial attempt and the retry truncate")
	}
	if !errors.Is(err, errTruncated) {
		t.Errorf("expected error to wrap errTruncated, got %v", err)
	}
}

// alwaysTruncatingChatter is a kronkChatter test double that always reports
// FinishReasonLength, for asserting cleanChunk gives up after one retry.
type alwaysTruncatingChatter struct{}

func (alwaysTruncatingChatter) ChatStreaming(ctx context.Context, d model.D) (<-chan model.ChatResponse, error) {
	lengthReason := model.FinishReasonLength
	ch := make(chan model.ChatResponse, 1)
	ch <- model.ChatResponse{Choices: []model.Choice{{
		Delta:           &model.ResponseMessage{Content: `{"blocks":[{"kind":"prose","text":"loop-`},
		FinishReasonPtr: &lengthReason,
	}}}
	close(ch)
	return ch, nil
}

// malformedThenSucceedingChatter is a kronkChatter test double simulating a
// finished (not truncated) but structurally broken JSON response on its
// first call and a valid response on the second -- reproduces a real
// production failure where grammar-constrained sampling produced malformed
// JSON without ever reporting FinishReasonLength.
type malformedThenSucceedingChatter struct {
	calls int
}

func (f *malformedThenSucceedingChatter) ChatStreaming(ctx context.Context, d model.D) (<-chan model.ChatResponse, error) {
	f.calls++
	ch := make(chan model.ChatResponse, 1)
	if f.calls == 1 {
		ch <- model.ChatResponse{Choices: []model.Choice{{
			Delta: &model.ResponseMessage{Content: "{\n \"blocks\": [\n {\n \"\t\t}\n ]\n }"},
		}}}
		close(ch)
		return ch, nil
	}

	resp := aiSegmentResult{Blocks: []aiSegmentedBlock{{Kind: "prose", Text: "recovered"}}}
	raw, _ := json.Marshal(resp)
	ch <- model.ChatResponse{Choices: []model.Choice{{Delta: &model.ResponseMessage{Content: string(raw)}}}}
	close(ch)
	return ch, nil
}

func TestAICleaner_CleanChunk_RetriesOnceAfterMalformedJSON(t *testing.T) {
	fake := &malformedThenSucceedingChatter{}
	c := &aiCleaner{
		cfg:    AIConfig{TimeoutBase: time.Second, Temperature: 0.2, RetryTemperatureBump: 0.4},
		client: fake,
	}
	c.initOnce.Do(func() {})

	got, err := c.Clean(context.Background(), []textRect{{Text: "x"}})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if fake.calls != 2 {
		t.Fatalf("expected 2 calls (initial + 1 retry), got %d", fake.calls)
	}
	if len(got) != 1 || got[0].Text != "recovered" {
		t.Errorf("expected the retry's successful result to be used, got %+v", got)
	}
}

func TestAICleaner_CleanChunk_FailsAfterRetryAlsoMalformed(t *testing.T) {
	c := &aiCleaner{
		cfg:    AIConfig{TimeoutBase: time.Second, Temperature: 0.2},
		client: &fakeChatter{content: "not json"},
	}
	c.initOnce.Do(func() {})

	_, err := c.Clean(context.Background(), []textRect{{Text: "x"}})
	if err == nil {
		t.Fatal("expected error when both the initial attempt and the retry produce malformed JSON")
	}
	if !errors.Is(err, errMalformed) {
		t.Errorf("expected error to wrap errMalformed, got %v", err)
	}
}
