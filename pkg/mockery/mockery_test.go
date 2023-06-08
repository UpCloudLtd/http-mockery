package mockery_test

import (
	"net/http"
	"testing"

	"github.com/UpCloudLtd/http-mockery/pkg/mockery"
	"github.com/stretchr/testify/assert"
)

func testingConfig() mockery.Config {
	return mockery.Config{
		Endpoints: []mockery.Endpoint{
			{
				Type:         mockery.EndpointTypeNormal,
				Uri:          "/example",
				Method:       "GET",
				ResponseCode: 200,
				Template:     "../../test/response-example.json",
				Variables: []mockery.Variable{
					{
						Name:  "item1",
						Value: "test123",
					},
				},
			},
			{
				Type:         mockery.EndpointTypeNormal,
				Uri:          "/resource",
				Method:       "POST",
				ResponseCode: 201,
				Template:     "../../test/response-resource-creation.json",
				Variables:    []mockery.Variable{},
			},
			{
				Type:         mockery.EndpointTypeRegex,
				Uri:          "/resource/[0-9]+",
				Method:       "DELETE",
				ResponseCode: 204,
			},
		},
	}
}

func TestConfigLoading(t *testing.T) {
	_, err := mockery.OpenConfigFile("test/nonexisting-config.json")
	assert.NotEqual(t, nil, err, "opening nonexistent config should cause an error")
	assert.ErrorContains(t, err, "no such file or directory", "error should be about file not existing")

	conf, err := mockery.OpenConfigFile("../../test/working-min-config.json")
	assert.Equal(t, nil, err, "working config should be opened without issues")

	assert.Equal(t, "1.2.3.4", conf.ListenIP, "IP should be set from config file")
	assert.Equal(t, 1337, conf.ListenPort, "Port should be set from config file")
	assert.Equal(t, 0, len(conf.Endpoints), "Minimal config should not have any configured endpoints")

	_, err = mockery.OpenConfigFile("../../test/broken-config.json")
	assert.NotEqual(t, nil, err, "opening broken config should cause an error")
	assert.ErrorContains(t, err, "invalid character", "error should be about invalid characters")
}

func TestConfigValidation(t *testing.T) {
	testMocker := mockery.MockHandler{
		Config: testingConfig(),
	}

	err := testMocker.ValidateConfig()
	assert.Equal(t, nil, err, "unmodified test config should have no errors")

	testMocker.Config.Endpoints[0].Method = "TRACE"
	err = testMocker.ValidateConfig()
	assert.ErrorContains(t, err, "Invalid HTTP method", "Config validation should fail on unsupported HTTP method")
	testMocker.Config = testingConfig()

	testMocker.Config.Endpoints[0].ResponseCode = 0
	err = testMocker.ValidateConfig()
	assert.ErrorContains(t, err, "must include response_code", "Config validation should fail on undefined response code")
}

func TestTemplateRendering(t *testing.T) {
	testMocker := mockery.MockHandler{
		Config: testingConfig(),
	}

	rendered, err := testMocker.RenderTemplateResponse(testMocker.Config.Endpoints[0])
	assert.Equal(t, nil, err, "Template should be rendered correctly")
	assert.Contains(t, rendered, testMocker.Config.Endpoints[0].Variables[0].Value, "Template should contain rendered value from config")
	assert.True(t, mockery.IsJSON(rendered), "Rendered template should be valid JSON")
}

func TestEndpointMatching(t *testing.T) {
	testMocker := mockery.MockHandler{
		Config: testingConfig(),
	}

	_, err := testMocker.MatchEndpoint(&http.Request{Method: http.MethodGet, RequestURI: "/example2"})
	assert.EqualError(t, err, mockery.ErrEndpointNotFound.Error(), "Should not match when URI is different")

	_, err = testMocker.MatchEndpoint(&http.Request{Method: http.MethodDelete, RequestURI: "/example"})
	assert.EqualError(t, err, mockery.ErrEndpointNotFound.Error(), "Should not match when HTTP method is different")

	endpoint, err := testMocker.MatchEndpoint(&http.Request{Method: http.MethodPost, RequestURI: "/resource"})
	assert.Equal(t, nil, err, "Endpoint should match")
	assert.Equal(t, testMocker.Config.Endpoints[1], endpoint, "Should be correctly matched endpoint")

	endpoint, err = testMocker.MatchEndpoint(&http.Request{Method: http.MethodDelete, RequestURI: "/resource/14354"})
	assert.Equal(t, nil, err, "Endpoint should match")
	assert.Equal(t, testMocker.Config.Endpoints[2], endpoint, "Should be correctly matched endpoint")
}
