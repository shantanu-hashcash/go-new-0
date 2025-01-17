package actions

import (
	"net/http"

	"github.com/shantanu-hashcash/go/support/render/problem"
)

// Well-known and reused problems below:
// inspired by similar default established in aurora - services/aurora/internal/render/problem/problem.go
var (

	// ClientDisconnected, represented by a non-standard HTTP status code of 499, which was introduced by
	// nginix.org(https://www.nginx.com/resources/wiki/extending/api/http/) as a way to capture this state.  Use it as a shortcut
	// in your actions.
	ClientDisconnected = problem.P{
		Type:   "client_disconnected",
		Title:  "Client Disconnected",
		Status: 499,
		Detail: "The client has closed the connection.",
	}

	// ServiceUnavailable is a well-known problem type.  Use it as a shortcut
	// in your actions.
	ServiceUnavailable = problem.P{
		Type:   "service_unavailable",
		Title:  "Service Unavailable",
		Status: http.StatusServiceUnavailable,
		Detail: "The request cannot be serviced at this time.",
	}

	// RateLimitExceeded is a well-known problem type.  Use it as a shortcut
	// in your actions.
	RateLimitExceeded = problem.P{
		Type:   "rate_limit_exceeded",
		Title:  "Rate Limit Exceeded",
		Status: 429,
		Detail: "The rate limit for the requesting IP address is over its alloted " +
			"limit.  The allowed limit and requests left per time period are " +
			"communicated to clients via the http response headers 'X-RateLimit-*' " +
			"headers.",
	}

	// NotImplemented is a well-known problem type.  Use it as a shortcut
	// in your actions.
	NotImplemented = problem.P{
		Type:   "not_implemented",
		Title:  "Resource Not Yet Implemented",
		Status: http.StatusNotFound,
		Detail: "While the requested URL is expected to eventually point to a " +
			"valid resource, the work to implement the resource has not yet " +
			"been completed.",
	}

	// NotAcceptable is a well-known problem type.  Use it as a shortcut
	// in your actions.
	NotAcceptable = problem.P{
		Type: "not_acceptable",
		Title: "An acceptable response content-type could not be provided for " +
			"this request",
		Status: http.StatusNotAcceptable,
	}

	// ServerOverCapacity is a well-known problem type.  Use it as a shortcut
	// in your actions.
	ServerOverCapacity = problem.P{
		Type:   "server_over_capacity",
		Title:  "Server Over Capacity",
		Status: http.StatusServiceUnavailable,
		Detail: "This aurora server is currently overloaded.  Please wait for " +
			"several minutes before trying your request again.",
	}

	// Timeout is a well-known problem type.  Use it as a shortcut
	// in your actions.
	Timeout = problem.P{
		Type:   "timeout",
		Title:  "Timeout",
		Status: http.StatusGatewayTimeout,
		Detail: "Your request timed out before completing.  Please try your " +
			"request again. If you are submitting a transaction make sure you are " +
			"sending exactly the same transaction (with the same sequence number).",
	}

	// UnsupportedMediaType is a well-known problem type.  Use it as a shortcut
	// in your actions.
	UnsupportedMediaType = problem.P{
		Type:   "unsupported_media_type",
		Title:  "Unsupported Media Type",
		Status: http.StatusUnsupportedMediaType,
		Detail: "The request has an unsupported content type. Presently, the " +
			"only supported content type is application/x-www-form-urlencoded.",
	}
)
