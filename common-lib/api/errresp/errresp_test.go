package errresp_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fab-viewer/common-lib/api/errresp"
	"github.com/fab-viewer/common-lib/common/bizerror"
)

func TestStatusFor(t *testing.T) {
	assert.Equal(t, http.StatusOK, errresp.StatusFor(bizerror.CodeOK))
	assert.Equal(t, http.StatusNotFound, errresp.StatusFor(bizerror.CodeNotFound))
	assert.Equal(t, http.StatusBadGateway, errresp.StatusFor(bizerror.CodeUpstream))
	assert.Equal(t, http.StatusInternalServerError, errresp.StatusFor(bizerror.Code("CUSTOM_X")),
		"未映射码按 500")
}
