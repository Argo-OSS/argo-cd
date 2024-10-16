package commands

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/util/localconfig"
)

const testConfig = `contexts:
- name: argocd1.example.com:443
  server: argocd1.example.com:443
  user: argocd1.example.com:443
- name: argocd2.example.com:443
  server: argocd2.example.com:443
  user: argocd2.example.com:443
- name: localhost:8080
  server: localhost:8080
  user: localhost:8080
current-context: localhost:8080
servers:
- server: argocd1.example.com:443
- server: argocd2.example.com:443
- plain-text: true
  server: localhost:8080
users:
- auth-token: vErrYS3c3tReFRe$hToken
  name: argocd1.example.com:443
  refresh-token: vErrYS3c3tReFRe$hToken
- auth-token: vErrYS3c3tReFRe$hToken
  name: argocd2.example.com:443
  refresh-token: vErrYS3c3tReFRe$hToken
- auth-token: vErrYS3c3tReFRe$hToken
  name: localhost:8080`

const testConfigFilePath = "./testdata/local.config"

func TestContextDelete(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)
	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)
	localConfig, err := localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	assert.Equal(t, "localhost:8080", localConfig.CurrentContext)
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "localhost:8080", Server: "localhost:8080", User: "localhost:8080"})

	// Delete a non-current context
	err = deleteContext("argocd1.example.com:443", testConfigFilePath)
	require.NoError(t, err)

	localConfig, err = localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	assert.Equal(t, "localhost:8080", localConfig.CurrentContext)
	assert.NotContains(t, localConfig.Contexts, localconfig.ContextRef{Name: "argocd1.example.com:443", Server: "argocd1.example.com:443", User: "argocd1.example.com:443"})
	assert.NotContains(t, localConfig.Servers, localconfig.Server{Server: "argocd1.example.com:443"})
	assert.NotContains(t, localConfig.Users, localconfig.User{AuthToken: "vErrYS3c3tReFRe$hToken", Name: "argocd1.example.com:443"})
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "argocd2.example.com:443", Server: "argocd2.example.com:443", User: "argocd2.example.com:443"})
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "localhost:8080", Server: "localhost:8080", User: "localhost:8080"})

	// Delete the current context
	err = deleteContext("localhost:8080", testConfigFilePath)
	require.NoError(t, err)

	localConfig, err = localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	assert.Equal(t, "", localConfig.CurrentContext)
	assert.NotContains(t, localConfig.Contexts, localconfig.ContextRef{Name: "localhost:8080", Server: "localhost:8080", User: "localhost:8080"})
	assert.NotContains(t, localConfig.Servers, localconfig.Server{PlainText: true, Server: "localhost:8080"})
	assert.NotContains(t, localConfig.Users, localconfig.User{AuthToken: "vErrYS3c3tReFRe$hToken", Name: "localhost:8080"})
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "argocd2.example.com:443", Server: "argocd2.example.com:443", User: "argocd2.example.com:443"})
}

func TestPrintArgoCDContexts(t *testing.T) {
	// Setup test configuration file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)
	defer os.Remove(testConfigFilePath)

	// Redirect os.Stdout to capture the output
	var buf bytes.Buffer
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	// Run printArgoCDContexts
	printArgoCDContexts(testConfigFilePath)

	// Restore os.Stdout and read the output
	w.Close()
	os.Stdout = origStdout
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "CURRENT\tNAME\tSERVER")
	assert.Contains(t, output, "*\tlocalhost:8080\tlocalhost:8080")
	assert.Contains(t, output, "\targocd1.example.com:443\targocd1.example.com:443")
	assert.Contains(t, output, "\targocd2.example.com:443\targocd2.example.com:443")
}

// Test for useArgoCDContext
func TestUseArgoCDContext(t *testing.T) {
	// Setup test configuration file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)
	defer os.Remove(testConfigFilePath)

	// Test switching to a different context
	err = useArgoCDContext("argocd2.example.com:443", testConfigFilePath)
	require.NoError(t, err)

	// Check that the context was updated
	localConfig, err := localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	assert.Equal(t, "argocd2.example.com:443", localConfig.CurrentContext)

	// Test switching back to the original context
	err = useArgoCDContext("localhost:8080", testConfigFilePath)
	require.NoError(t, err)
	localConfig, err = localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	assert.Equal(t, "localhost:8080", localConfig.CurrentContext)
}
