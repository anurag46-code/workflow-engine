package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/smtp"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/anurag46-code/workflow-engine/internal/engine"
	"github.com/anurag46-code/workflow-engine/internal/models"
	"github.com/anurag46-code/workflow-engine/internal/store"
	"github.com/google/uuid"
)

const (
	pollInterval  = 2 * time.Second
	leaseDuration = 30 * time.Second
)

type Worker struct {
	id     string
	store  *store.Store
	engine *engine.Engine
}

func New(s *store.Store, eng *engine.Engine) *Worker {
	return &Worker{
		id:     "worker-" + uuid.New().String()[:8],
		store:  s,
		engine: eng,
	}
}

func (w *Worker) Run() {
	log.Printf("[%s] started", w.id)
	for {
		task, err := w.store.ClaimTask(w.id, leaseDuration)
		if err != nil {
			log.Printf("[%s] claim error: %v", w.id, err)
			time.Sleep(pollInterval)
			continue
		}
		if task == nil {
			time.Sleep(pollInterval)
			continue
		}
		w.execute(task)
	}
}

func (w *Worker) execute(task *models.TaskRun) {
	log.Printf("[%s] executing task %s (type=%s)", w.id, task.TaskID, task.Type)

	// Fetch upstream outputs so tasks can pass data down the pipeline
	upstream, err := w.store.GetUpstreamOutputs(task.WorkflowID, task.DependsOn)
	if err != nil {
		log.Printf("[%s] get upstream outputs: %v", w.id, err)
		upstream = map[string]string{}
	}

	output, execErr := w.dispatch(task, upstream)
	if execErr != nil {
		log.Printf("[%s] task %s failed (attempt %d/%d): %v", w.id, task.TaskID, task.Retries+1, task.MaxRetries, execErr)
		if storeErr := w.store.FailTask(task.ID, execErr.Error(), task.Retries, task.MaxRetries); storeErr != nil {
			log.Printf("[%s] fail store error: %v", w.id, storeErr)
		}
	} else {
		log.Printf("[%s] task %s succeeded", w.id, task.TaskID)
		if storeErr := w.store.CompleteTask(task.ID, output); storeErr != nil {
			log.Printf("[%s] complete store error: %v", w.id, storeErr)
		}
	}

	w.engine.Advance(task.WorkflowID)
}

func (w *Worker) dispatch(task *models.TaskRun, upstream map[string]string) (string, error) {
	switch task.Type {
	case "ingest":
		return w.runIngest(task)
	case "word_count":
		return w.runWordCount(task, upstream)
	case "extract_keywords":
		return w.runExtractKeywords(task, upstream)
	case "detect_sentiment":
		return w.runDetectSentiment(task, upstream)
	case "generate_report":
		return w.runGenerateReport(task, upstream)
	case "send_email":
		return w.runSendEmail(task, upstream)
	// legacy types kept for custom workflow builder
	case "wait":
		return w.runWait(task)
	case "transform":
		return w.runTransform(task, upstream)
	case "fail_sometimes":
		return w.runFailSometimes(task)
	default:
		return "", fmt.Errorf("unknown task type: %s", task.Type)
	}
}

// --- Text Analysis Pipeline handlers ---

// runIngest validates and normalises the input text.
func (w *Worker) runIngest(task *models.TaskRun) (string, error) {
	text, _ := task.Config["text"].(string)
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("ingest: text is empty")
	}
	text = strings.TrimSpace(text)
	wordCount := len(strings.Fields(text))
	log.Printf("[%s] ingested %d words", w.id, wordCount)
	// Output is the cleaned text - passed to all downstream tasks
	return text, nil
}

// runWordCount counts words, sentences, paragraphs and avg word length.
func (w *Worker) runWordCount(task *models.TaskRun, upstream map[string]string) (string, error) {
	text := upstreamText(upstream)
	if text == "" {
		return "", fmt.Errorf("word_count: no upstream text")
	}
	time.Sleep(time.Duration(800+rand.Intn(600)) * time.Millisecond)

	words := strings.Fields(text)
	sentences := strings.Count(text, ".") + strings.Count(text, "!") + strings.Count(text, "?")
	if sentences == 0 {
		sentences = 1
	}
	paragraphs := len(strings.Split(strings.TrimSpace(text), "\n\n"))

	totalLen := 0
	for _, w := range words {
		totalLen += len(strings.Trim(w, ".,!?;:\"'"))
	}
	avgLen := 0.0
	if len(words) > 0 {
		avgLen = float64(totalLen) / float64(len(words))
	}

	result := map[string]any{
		"words":      len(words),
		"sentences":  sentences,
		"paragraphs": paragraphs,
		"avgWordLen": fmt.Sprintf("%.1f", avgLen),
		"chars":      len(text),
	}
	b, _ := json.Marshal(result)
	return string(b), nil
}

// runExtractKeywords finds the top N most frequent non-stopword terms.
func (w *Worker) runExtractKeywords(task *models.TaskRun, upstream map[string]string) (string, error) {
	text := upstreamText(upstream)
	if text == "" {
		return "", fmt.Errorf("extract_keywords: no upstream text")
	}
	time.Sleep(time.Duration(600+rand.Intn(800)) * time.Millisecond)

	stopwords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "is": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"that": true, "this": true, "it": true, "its": true, "as": true, "by": true,
		"from": true, "not": true, "no": true, "so": true, "if": true, "i": true,
		"you": true, "he": true, "she": true, "we": true, "they": true, "their": true,
	}

	freq := map[string]int{}
	for _, word := range strings.Fields(strings.ToLower(text)) {
		clean := strings.TrimFunc(word, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		if len(clean) > 3 && !stopwords[clean] {
			freq[clean]++
		}
	}

	type kv struct {
		word  string
		count int
	}
	var sorted []kv
	for k, v := range freq {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	top := 8
	if len(sorted) < top {
		top = len(sorted)
	}
	keywords := make([]string, top)
	for i := 0; i < top; i++ {
		keywords[i] = sorted[i].word
	}

	b, _ := json.Marshal(map[string]any{"keywords": keywords})
	return string(b), nil
}

// runDetectSentiment does simple lexicon-based sentiment scoring.
func (w *Worker) runDetectSentiment(task *models.TaskRun, upstream map[string]string) (string, error) {
	text := upstreamText(upstream)
	if text == "" {
		return "", fmt.Errorf("detect_sentiment: no upstream text")
	}
	time.Sleep(time.Duration(500+rand.Intn(700)) * time.Millisecond)

	positive := map[string]bool{
		"good": true, "great": true, "excellent": true, "amazing": true, "wonderful": true,
		"fantastic": true, "outstanding": true, "positive": true, "success": true, "happy": true,
		"love": true, "best": true, "perfect": true, "brilliant": true, "superb": true,
		"efficient": true, "effective": true, "innovative": true, "impressive": true,
	}
	negative := map[string]bool{
		"bad": true, "terrible": true, "awful": true, "horrible": true, "negative": true,
		"failure": true, "fail": true, "poor": true, "worst": true, "hate": true,
		"problem": true, "issue": true, "error": true, "wrong": true, "difficult": true,
		"slow": true, "broken": true, "useless": true, "disappointing": true,
	}

	posScore, negScore := 0, 0
	for _, word := range strings.Fields(strings.ToLower(text)) {
		clean := strings.TrimFunc(word, func(r rune) bool { return !unicode.IsLetter(r) })
		if positive[clean] {
			posScore++
		}
		if negative[clean] {
			negScore++
		}
	}

	label := "neutral"
	if posScore > negScore+1 {
		label = "positive"
	} else if negScore > posScore+1 {
		label = "negative"
	}

	b, _ := json.Marshal(map[string]any{
		"sentiment":  label,
		"posScore":   posScore,
		"negScore":   negScore,
		"confidence": fmt.Sprintf("%.0f%%", float64(max(posScore, negScore)+1)/float64(posScore+negScore+2)*100),
	})
	return string(b), nil
}

// runGenerateReport combines all upstream task outputs into a formatted report.
func (w *Worker) runGenerateReport(task *models.TaskRun, upstream map[string]string) (string, error) {
	time.Sleep(500 * time.Millisecond)

	var wc, kw, sent map[string]any
	for _, out := range upstream {
		var m map[string]any
		if err := json.Unmarshal([]byte(out), &m); err != nil {
			continue
		}
		if _, ok := m["words"]; ok {
			wc = m
		}
		if _, ok := m["keywords"]; ok {
			kw = m
		}
		if _, ok := m["sentiment"]; ok {
			sent = m
		}
	}

	var buf bytes.Buffer
	buf.WriteString("=== TEXT ANALYSIS REPORT ===\n\n")

	if wc != nil {
		buf.WriteString(fmt.Sprintf("STATISTICS\n  Words: %v | Sentences: %v | Paragraphs: %v\n  Avg word length: %v chars | Total chars: %v\n\n",
			wc["words"], wc["sentences"], wc["paragraphs"], wc["avgWordLen"], wc["chars"]))
	}
	if kw != nil {
		if kwList, ok := kw["keywords"].([]any); ok {
			words := make([]string, len(kwList))
			for i, k := range kwList {
				words[i] = fmt.Sprint(k)
			}
			buf.WriteString(fmt.Sprintf("TOP KEYWORDS\n  %s\n\n", strings.Join(words, ", ")))
		}
	}
	if sent != nil {
		buf.WriteString(fmt.Sprintf("SENTIMENT\n  Label: %v | Confidence: %v\n  Positive signals: %v | Negative signals: %v\n\n",
			sent["sentiment"], sent["confidence"], sent["posScore"], sent["negScore"]))
	}
	buf.WriteString("Generated by Workflow Engine\n")
	return buf.String(), nil
}

// runSendEmail sends the report via SMTP to Mailhog (local) or real SMTP.
func (w *Worker) runSendEmail(task *models.TaskRun, upstream map[string]string) (string, error) {
	to, _ := task.Config["to"].(string)
	if to == "" {
		to = "demo@example.com"
	}

	// Collect the report from upstream generate_report output
	report := ""
	for _, out := range upstream {
		if strings.Contains(out, "TEXT ANALYSIS REPORT") {
			report = out
			break
		}
	}
	if report == "" {
		report = "No report generated."
	}

	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	if smtpHost == "" {
		smtpHost = "mailhog"
	}
	if smtpPort == "" {
		smtpPort = "1025"
	}

	from := "workflow-engine@localhost"
	subject := "Text Analysis Report"
	body := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, report)

	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	err := smtp.SendMail(addr, nil, from, []string{to}, []byte(body))
	if err != nil {
		return "", fmt.Errorf("smtp: %w", err)
	}
	return fmt.Sprintf("email sent to %s via %s", to, addr), nil
}

// --- Legacy handlers for custom workflow builder ---

func (w *Worker) runWait(task *models.TaskRun) (string, error) {
	secs := 2.0
	if v, ok := task.Config["seconds"].(float64); ok {
		secs = v
	}
	jitter := time.Duration(rand.Intn(800)) * time.Millisecond
	time.Sleep(time.Duration(secs*float64(time.Second)) + jitter)
	return fmt.Sprintf("waited %.1fs", secs), nil
}

func (w *Worker) runTransform(task *models.TaskRun, upstream map[string]string) (string, error) {
	input, _ := task.Config["input"].(string)
	if input == "" {
		input = upstreamText(upstream)
	}
	return fmt.Sprintf("transformed: %s", strings.ToUpper(input)), nil
}

func (w *Worker) runFailSometimes(task *models.TaskRun) (string, error) {
	failRate := 0.6
	if v, ok := task.Config["failRate"].(float64); ok {
		failRate = v
	}
	if task.Retries == 0 && rand.Float64() < failRate {
		return "", fmt.Errorf("simulated failure (retry to succeed)")
	}
	return "succeeded after retry", nil
}

// upstreamText returns the first upstream output that looks like plain text (not JSON).
func upstreamText(upstream map[string]string) string {
	for _, out := range upstream {
		if !strings.HasPrefix(strings.TrimSpace(out), "{") {
			return out
		}
	}
	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
