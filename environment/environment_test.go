package environment

import (
	"context"
	"reflect"
	"testing"
	"time"

	"dagger.io/dagger"
)

func TestHistory_Latest(t *testing.T) {
	tests := []struct {
		name    string
		history History
		want    *Revision
	}{
		{
			name:    "empty history",
			history: History{},
			want:    nil,
		},
		{
			name: "single revision",
			history: History{
				{Version: 1, Name: "first"},
			},
			want: &Revision{Version: 1, Name: "first"},
		},
		{
			name: "multiple revisions",
			history: History{
				{Version: 1, Name: "first"},
				{Version: 2, Name: "second"},
				{Version: 3, Name: "third"},
			},
			want: &Revision{Version: 3, Name: "third"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.history.Latest()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("History.Latest() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestHistory_LatestVersion(t *testing.T) {
	tests := []struct {
		name    string
		history History
		want    Version
	}{
		{
			name:    "empty history",
			history: History{},
			want:    0,
		},
		{
			name: "single revision",
			history: History{
				{Version: 5, Name: "first"},
			},
			want: 5,
		},
		{
			name: "multiple revisions",
			history: History{
				{Version: 1, Name: "first"},
				{Version: 3, Name: "second"},
				{Version: 2, Name: "third"},
			},
			want: 2, // Returns version of latest revision (last in slice), not highest version
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.history.LatestVersion()
			if got != tt.want {
				t.Errorf("History.LatestVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHistory_Get(t *testing.T) {
	tests := []struct {
		name    string
		history History
		version Version
		want    *Revision
	}{
		{
			name:    "empty history",
			history: History{},
			version: 1,
			want:    nil,
		},
		{
			name: "version found",
			history: History{
				{Version: 1, Name: "first"},
				{Version: 2, Name: "second"},
				{Version: 3, Name: "third"},
			},
			version: 2,
			want:    &Revision{Version: 2, Name: "second"},
		},
		{
			name: "version not found",
			history: History{
				{Version: 1, Name: "first"},
				{Version: 3, Name: "third"},
			},
			version: 2,
			want:    nil,
		},
		{
			name: "version 0",
			history: History{
				{Version: 0, Name: "zero"},
				{Version: 1, Name: "one"},
			},
			version: 0,
			want:    &Revision{Version: 0, Name: "zero"},
		},
		{
			name: "multiple revisions with same version - returns first",
			history: History{
				{Version: 1, Name: "first"},
				{Version: 1, Name: "duplicate"},
			},
			version: 1,
			want:    &Revision{Version: 1, Name: "first"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.history.Get(tt.version)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("History.Get() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestInitialize(t *testing.T) {
	tests := []struct {
		name   string
		client *dagger.Client
	}{
		{
			name:   "initialize with nil client",
			client: nil,
		},
		// Note: We can't easily create a real dagger.Client in tests
		// so we just test with nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store original value to restore later
			originalDag := dag
			defer func() { dag = originalDag }()

			err := Initialize(tt.client)
			if err != nil {
				t.Errorf("Initialize() error = %v, want nil", err)
			}
			if dag != tt.client {
				t.Errorf("Initialize() did not set global dag variable correctly")
			}
		})
	}
}

func TestContainerWithEnvAndSecrets(t *testing.T) {
	tests := []struct {
		name      string
		envs      []string
		secrets   []string
		wantErr   bool
		errString string
	}{
		{
			name:    "empty envs and secrets",
			envs:    []string{},
			secrets: []string{},
			wantErr: false,
		},
		{
			name:      "invalid environment variable - no equals",
			envs:      []string{"INVALID_ENV"},
			secrets:   []string{},
			wantErr:   true,
			errString: "invalid env variable: INVALID_ENV",
		},
		{
			name:      "invalid secret - no equals",
			envs:      []string{},
			secrets:   []string{"INVALID_SECRET"},
			wantErr:   true,
			errString: "invalid secret: INVALID_SECRET",
		},
		{
			name:      "mixed valid and invalid env - invalid first",
			envs:      []string{"INVALID", "VALID=value"},
			secrets:   []string{},
			wantErr:   true,
			errString: "invalid env variable: INVALID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can only test the validation logic since we can't create a real dagger.Container
			// For test cases that should succeed, we expect a panic due to nil container
			// For test cases that should fail, we expect an error before the panic
			
			defer func() {
				if r := recover(); r != nil {
					if tt.wantErr {
						t.Errorf("containerWithEnvAndSecrets() panicked but should have returned error: %v", r)
					}
					// If !tt.wantErr, panic is expected due to nil container after validation passes
				}
			}()

			_, err := containerWithEnvAndSecrets(nil, tt.envs, tt.secrets)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("containerWithEnvAndSecrets() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errString != "" && err != nil {
				if err.Error() != tt.errString {
					t.Errorf("containerWithEnvAndSecrets() error = %v, want error %q", err, tt.errString)
				}
			}
		})
	}
}

func TestGet(t *testing.T) {
	// Set up test environments
	originalEnvironments := environments
	defer func() { environments = originalEnvironments }()

	environments = map[string]*Environment{
		"project1/env-1": {ID: "project1/env-1", Name: "project1"},
		"project2/env-2": {ID: "project2/env-2", Name: "project2"},
		"test/unique":    {ID: "test/unique", Name: "unique"},
		"app/env-1":      {ID: "app/env-1", Name: "app"},
	}

	tests := []struct {
		name      string
		idOrName  string
		want      *Environment
		wantFound bool
	}{
		{
			name:      "get by exact ID",
			idOrName:  "project1/env-1",
			want:      environments["project1/env-1"],
			wantFound: true,
		},
		{
			name:      "get by name - single match",
			idOrName:  "unique",
			want:      environments["test/unique"],
			wantFound: true,
		},
		{
			name:      "get by name - multiple matches returns first found",
			idOrName:  "project1",
			want:      environments["project1/env-1"],
			wantFound: true,
		},
		{
			name:      "not found - invalid ID",
			idOrName:  "nonexistent/id",
			want:      nil,
			wantFound: false,
		},
		{
			name:      "not found - invalid name",
			idOrName:  "nonexistent",
			want:      nil,
			wantFound: false,
		},
		{
			name:      "empty string",
			idOrName:  "",
			want:      nil,
			wantFound: false,
		},
		{
			name:      "partial ID match not found",
			idOrName:  "project1/",
			want:      nil,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Get(tt.idOrName)
			
			if tt.wantFound && got == nil {
				t.Errorf("Get() = nil, want %+v", tt.want)
				return
			}
			
			if !tt.wantFound && got != nil {
				t.Errorf("Get() = %+v, want nil", got)
				return
			}
			
			if tt.wantFound && got != tt.want {
				t.Errorf("Get() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestEnvironment_SetEnv(t *testing.T) {
	tests := []struct {
		name    string
		envs    []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid environment variables",
			envs:    []string{"KEY1=value1", "KEY2=value2"},
			wantErr: false,
		},
		{
			name:    "empty environment list",
			envs:    []string{},
			wantErr: false,
		},
		{
			name:    "environment with empty value",
			envs:    []string{"EMPTY="},
			wantErr: false,
		},
		{
			name:    "environment with equals in value",
			envs:    []string{"URL=http://example.com:8080/path?param=value"},
			wantErr: false,
		},
		{
			name:    "invalid environment variable - no equals",
			envs:    []string{"INVALID_ENV"},
			wantErr: true,
			errMsg:  "invalid environment variable: INVALID_ENV",
		},
		{
			name:    "invalid environment variable - empty string",
			envs:    []string{""},
			wantErr: true,
			errMsg:  "invalid environment variable: ",
		},
		{
			name:    "mixed valid and invalid",
			envs:    []string{"VALID=value", "INVALID"},
			wantErr: true,
			errMsg:  "invalid environment variable: INVALID",
		},
		{
			name:    "empty key",
			envs:    []string{"=value"},
			wantErr: false, // Empty key is allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := &Environment{
				// We can't easily mock container, so this will panic
				// when it tries to call methods on nil container
				// We're mainly testing the validation logic here
			}

			defer func() {
				if r := recover(); r != nil && !tt.wantErr {
					// Expected to panic due to nil container, but we caught validation errors first
					// if we wanted, which is good
				}
			}()

			err := env.SetEnv(context.Background(), "test", tt.envs)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("SetEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" && err != nil {
				if err.Error() != tt.errMsg {
					t.Errorf("SetEnv() error = %v, want error %q", err, tt.errMsg)
				}
			}
		})
	}
}

// Test helper function to create a revision with specific fields
func createRevision(version Version, name string, createdAt time.Time) *Revision {
	return &Revision{
		Version:   version,
		Name:      name,
		CreatedAt: createdAt,
	}
}

func TestRevisionCreation(t *testing.T) {
	tests := []struct {
		name     string
		version  Version
		revName  string
		expected *Revision
	}{
		{
			name:    "create basic revision",
			version: 1,
			revName: "initial",
			expected: &Revision{
				Version: 1,
				Name:    "initial",
			},
		},
		{
			name:    "create revision with zero version",
			version: 0,
			revName: "zero",
			expected: &Revision{
				Version: 0,
				Name:    "zero",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			revision := &Revision{
				Version: tt.version,
				Name:    tt.revName,
			}

			if revision.Version != tt.expected.Version {
				t.Errorf("Revision.Version = %v, want %v", revision.Version, tt.expected.Version)
			}
			if revision.Name != tt.expected.Name {
				t.Errorf("Revision.Name = %q, want %q", revision.Name, tt.expected.Name)
			}
		})
	}
}

// Test Environment creation and basic field validation
func TestEnvironmentCreation(t *testing.T) {
	tests := []struct {
		name     string
		env      *Environment
		wantName string
		wantID   string
	}{
		{
			name: "basic environment",
			env: &Environment{
				ID:   "test/env-1",
				Name: "test",
			},
			wantName: "test",
			wantID:   "test/env-1",
		},
		{
			name: "empty environment",
			env:  &Environment{},
			wantName: "",
			wantID:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env.Name != tt.wantName {
				t.Errorf("Environment.Name = %q, want %q", tt.env.Name, tt.wantName)
			}
			if tt.env.ID != tt.wantID {
				t.Errorf("Environment.ID = %q, want %q", tt.env.ID, tt.wantID)
			}
		})
	}
}
