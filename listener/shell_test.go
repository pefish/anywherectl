package listener

import (
	"github.com/pefish/anywherectl/internal/test"
	"testing"
)

func TestExecShell(t *testing.T) {
	result, err := ExecShell(`echo "test"`)
	test.Nil(t, err)
	test.Equal(t, "test\n", result)

	result1, err := ExecShell(`tetetet && echo $?`)
	test.NotNil(t, err)
	test.Equal(t, "exit status 127", err.Error())
	test.Equal(t, "", result1)

	result2, err := ExecShell(`echo "test" && echo $?`)
	test.Nil(t, err)
	test.Equal(t, "test\n0\n", result2)

}

