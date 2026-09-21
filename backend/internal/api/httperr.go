package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type clientError struct {
	code int
	msg  string
}

func (e clientError) Error() string { return e.msg }

func badRequest(msg string) error { return clientError{code: http.StatusBadRequest, msg: msg} }

func notFound(msg string) error { return clientError{code: http.StatusNotFound, msg: msg} }

func writeErr(c *gin.Context, err error) {
	if err == nil {
		return
	}
	var ce clientError
	if errors.As(err, &ce) {
		code := ce.code
		if code == 0 {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": ce.msg})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
