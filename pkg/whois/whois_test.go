package whois

import (
	"encoding/json"
	"github.com/likexian/gokit/assert"
	"testing"
)

func TestExecute(t *testing.T) {
	query := &Query{Domain: "baidu.com"}
	execute, err := Execute(query)
	assert.Nil(t, err)
	assert.NotNil(t, execute)
	marshal, _ := json.Marshal(execute)
	t.Log(string(marshal))
}
