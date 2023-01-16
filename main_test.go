package help

import (
	"context"
	"io"
	"testing"
)

func TestTopicTemplate(t *testing.T) {
	topic := HelpTopic{
		Name: "test",
	}

	w := io.Discard

	err := tmpl().ExecuteTemplate(w, "topic.html", topicPageData{
		Context:   context.Background(),
		HelpTopic: topic,
	})
	if err != nil {
		t.FailNow()
	}
}
