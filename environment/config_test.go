package environment

import (
	"encoding/json"
	"os"
	"path"
	"reflect"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	expected := &EnvironmentConfig{
		BaseImage:    defaultImage,
		Instructions: "No instructions found. Please look around the filesystem and update me",
		Workdir:      "/workdir",
	}
	
	if !reflect.DeepEqual(config, expected) {
		t.Errorf("DefaultConfig() = %+v, want %+v", config, expected)
	}
}

func TestServiceConfigs_Get(t *testing.T) {
	tests := []struct {
		name     string
		services ServiceConfigs
		lookup   string
		want     *ServiceConfig
	}{
		{
			name:     "empty services",
			services: ServiceConfigs{},
			lookup:   "test",
			want:     nil,
		},
		{
			name: "service found",
			services: ServiceConfigs{
				{Name: "web", Image: "nginx"},
				{Name: "db", Image: "postgres"},
			},
			lookup: "web",
			want:   &ServiceConfig{Name: "web", Image: "nginx"},
		},
		{
			name: "service not found",
			services: ServiceConfigs{
				{Name: "web", Image: "nginx"},
				{Name: "db", Image: "postgres"},
			},
			lookup: "cache",
			want:   nil,
		},
		{
			name: "multiple services with same name - returns first",
			services: ServiceConfigs{
				{Name: "web", Image: "nginx:1.0"},
				{Name: "web", Image: "nginx:2.0"},
			},
			lookup: "web",
			want:   &ServiceConfig{Name: "web", Image: "nginx:1.0"},
		},
		{
			name: "case sensitive lookup",
			services: ServiceConfigs{
				{Name: "Web", Image: "nginx"},
			},
			lookup: "web",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.services.Get(tt.lookup)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ServiceConfigs.Get() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestEnvironmentConfig_Copy(t *testing.T) {
	tests := []struct {
		name   string
		config *EnvironmentConfig
	}{
		{
			name: "basic config",
			config: &EnvironmentConfig{
				Instructions:  "test instructions",
				Workdir:      "/test",
				BaseImage:    "test:latest",
				SetupCommands: []string{"apt update", "apt install -y curl"},
				Env:          []string{"ENV=test", "DEBUG=true"},
				Secrets:      []string{"SECRET1", "SECRET2"},
				Services:     ServiceConfigs{}, // Initialize as empty slice, not nil
			},
		},
		{
			name: "config with services",
			config: &EnvironmentConfig{
				Instructions: "test instructions",
				Workdir:     "/test",
				BaseImage:   "test:latest",
				Services: ServiceConfigs{
					{
						Name:         "web",
						Image:        "nginx",
						Command:      "nginx -g 'daemon off;'",
						ExposedPorts: []int{80, 443},
						Env:          []string{"NGINX_HOST=localhost"},
						Secrets:      []string{"SSL_CERT"},
					},
					{
						Name:         "db",
						Image:        "postgres:13",
						Command:      "postgres",
						ExposedPorts: []int{5432},
						Env:          []string{"POSTGRES_DB=test"},
						Secrets:      []string{"POSTGRES_PASSWORD"},
					},
				},
			},
		},
		{
			name: "empty config",
			config: &EnvironmentConfig{
				Services: ServiceConfigs{}, // Initialize as empty slice, not nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.config
			copied := original.Copy()

			// Test that the copy has the same content as the original
			// Note: Copy() always creates a new slice, so nil Services becomes empty slice
			expectedCopy := *original
			if original.Services == nil {
				expectedCopy.Services = ServiceConfigs{}
			}
			
			if !reflect.DeepEqual(copied, &expectedCopy) {
				t.Errorf("Copy() result not as expected:\ncopied: %+v\nexpected: %+v", copied, &expectedCopy)
			}

			// Test that the copy is a different instance
			if original == copied {
				t.Error("Copy() returned the same instance, not a copy")
			}

			// Test that services slice is a deep copy
			if len(original.Services) > 0 {
				if &original.Services[0] == &copied.Services[0] {
					t.Error("Copy() did not deep copy services slice")
				}
			}

			// Test that modifying the copy doesn't affect the original
			copied.Instructions = "modified"
			if original.Instructions == copied.Instructions {
				t.Error("Modifying copy affected the original")
			}

			// Test that modifying service in copy doesn't affect original
			if len(copied.Services) > 0 {
				originalServiceName := ""
				if len(original.Services) > 0 {
					originalServiceName = original.Services[0].Name
				}
				
				copied.Services[0].Name = "modified"
				
				if len(original.Services) > 0 && original.Services[0].Name != originalServiceName {
					t.Error("Modifying service in copy affected the original")
				}
			}
		})
	}
}

func TestEnvironmentConfig_Save(t *testing.T) {
	tests := []struct {
		name    string
		config  *EnvironmentConfig
		wantErr bool
	}{
		{
			name: "basic save",
			config: &EnvironmentConfig{
				Instructions:  "test instructions",
				Workdir:      "/test",
				BaseImage:    "test:latest",
				SetupCommands: []string{"echo test"},
				Env:          []string{"TEST=value"},
				Secrets:      []string{"SECRET"},
			},
			wantErr: false,
		},
		{
			name: "save with services",
			config: &EnvironmentConfig{
				Instructions: "test with services",
				Services: ServiceConfigs{
					{
						Name:         "web",
						Image:        "nginx",
						ExposedPorts: []int{80},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty config",
			config:  &EnvironmentConfig{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory
			tempDir := t.TempDir()

			err := tt.config.Save(tempDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnvironmentConfig.Save() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify that the config directory was created
				configPath := path.Join(tempDir, configDir)
				if _, err := os.Stat(configPath); os.IsNotExist(err) {
					t.Errorf("Config directory was not created: %s", configPath)
				}

				// Verify that the instructions file was created and has correct content
				instructionsPath := path.Join(configPath, instructionsFile)
				instructionsData, err := os.ReadFile(instructionsPath)
				if err != nil {
					t.Errorf("Failed to read instructions file: %v", err)
				}
				if string(instructionsData) != tt.config.Instructions {
					t.Errorf("Instructions file content = %q, want %q", string(instructionsData), tt.config.Instructions)
				}

				// Verify that the environment file was created and has correct content
				envPath := path.Join(configPath, environmentFile)
				envData, err := os.ReadFile(envPath)
				if err != nil {
					t.Errorf("Failed to read environment file: %v", err)
				}

				// Parse the JSON and compare
				var savedConfig EnvironmentConfig
				if err := json.Unmarshal(envData, &savedConfig); err != nil {
					t.Errorf("Failed to parse environment file JSON: %v", err)
				}

				// Compare relevant fields (Instructions is not saved in JSON)
				expectedConfig := *tt.config
				expectedConfig.Instructions = "" // Instructions field is not saved in JSON
				savedConfig.Instructions = ""    // Clear for comparison

				if !reflect.DeepEqual(savedConfig, expectedConfig) {
					t.Errorf("Saved config = %+v, want %+v", savedConfig, expectedConfig)
				}
			}
		})
	}
}

func TestEnvironmentConfig_Load(t *testing.T) {
	tests := []struct {
		name         string
		setupFiles   func(string) error
		wantErr      bool
		wantConfig   *EnvironmentConfig
		errorContains string
	}{
		{
			name: "successful load",
			setupFiles: func(baseDir string) error {
				configPath := path.Join(baseDir, configDir)
				if err := os.MkdirAll(configPath, 0755); err != nil {
					return err
				}

				// Create instructions file
				instructions := "test instructions for loading"
				if err := os.WriteFile(path.Join(configPath, instructionsFile), []byte(instructions), 0644); err != nil {
					return err
				}

				// Create environment file
				config := EnvironmentConfig{
					Workdir:      "/test/workdir",
					BaseImage:    "test:latest",
					SetupCommands: []string{"echo test"},
					Env:          []string{"TEST=value"},
					Services: ServiceConfigs{
						{Name: "web", Image: "nginx"},
					},
				}
				data, _ := json.MarshalIndent(config, "", "  ")
				return os.WriteFile(path.Join(configPath, environmentFile), data, 0644)
			},
			wantErr: false,
			wantConfig: &EnvironmentConfig{
				Instructions:  "test instructions for loading",
				Workdir:      "/test/workdir",
				BaseImage:    "test:latest",
				SetupCommands: []string{"echo test"},
				Env:          []string{"TEST=value"},
				Services: ServiceConfigs{
					{Name: "web", Image: "nginx"},
				},
			},
		},
		{
			name: "missing instructions file",
			setupFiles: func(baseDir string) error {
				configPath := path.Join(baseDir, configDir)
				if err := os.MkdirAll(configPath, 0755); err != nil {
					return err
				}
				// Only create environment file, not instructions file
				config := EnvironmentConfig{Workdir: "/test"}
				data, _ := json.MarshalIndent(config, "", "  ")
				return os.WriteFile(path.Join(configPath, environmentFile), data, 0644)
			},
			wantErr:       true,
			errorContains: "no such file or directory",
		},
		{
			name: "missing environment file",
			setupFiles: func(baseDir string) error {
				configPath := path.Join(baseDir, configDir)
				if err := os.MkdirAll(configPath, 0755); err != nil {
					return err
				}
				// Only create instructions file, not environment file
				return os.WriteFile(path.Join(configPath, instructionsFile), []byte("test"), 0644)
			},
			wantErr:       true,
			errorContains: "no such file or directory",
		},
		{
			name: "invalid JSON in environment file",
			setupFiles: func(baseDir string) error {
				configPath := path.Join(baseDir, configDir)
				if err := os.MkdirAll(configPath, 0755); err != nil {
					return err
				}

				if err := os.WriteFile(path.Join(configPath, instructionsFile), []byte("test"), 0644); err != nil {
					return err
				}

				// Write invalid JSON
				return os.WriteFile(path.Join(configPath, environmentFile), []byte("invalid json"), 0644)
			},
			wantErr:       true,
			errorContains: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			if err := tt.setupFiles(tempDir); err != nil {
				t.Fatalf("Failed to setup test files: %v", err)
			}

			config := &EnvironmentConfig{}
			err := config.Load(tempDir)

			if (err != nil) != tt.wantErr {
				t.Errorf("EnvironmentConfig.Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errorContains != "" {
				if err == nil || !contains(err.Error(), tt.errorContains) {
					t.Errorf("EnvironmentConfig.Load() error = %v, want error containing %q", err, tt.errorContains)
				}
			}

			if !tt.wantErr && tt.wantConfig != nil {
				if !reflect.DeepEqual(config, tt.wantConfig) {
					t.Errorf("EnvironmentConfig.Load() loaded config = %+v, want %+v", config, tt.wantConfig)
				}
			}
		})
	}
}

func TestEnvironmentConfig_Locked(t *testing.T) {
	tests := []struct {
		name      string
		setupLock func(string) error
		want      bool
	}{
		{
			name: "not locked - no lock file",
			setupLock: func(baseDir string) error {
				// Don't create any files
				return nil
			},
			want: false,
		},
		{
			name: "not locked - no config directory",
			setupLock: func(baseDir string) error {
				// Don't create the config directory
				return nil
			},
			want: false,
		},
		{
			name: "locked - lock file exists",
			setupLock: func(baseDir string) error {
				configPath := path.Join(baseDir, configDir)
				if err := os.MkdirAll(configPath, 0755); err != nil {
					return err
				}
				return os.WriteFile(path.Join(configPath, lockFile), []byte(""), 0644)
			},
			want: true,
		},
		{
			name: "locked - lock file exists with content",
			setupLock: func(baseDir string) error {
				configPath := path.Join(baseDir, configDir)
				if err := os.MkdirAll(configPath, 0755); err != nil {
					return err
				}
				return os.WriteFile(path.Join(configPath, lockFile), []byte("lock content"), 0644)
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			if err := tt.setupLock(tempDir); err != nil {
				t.Fatalf("Failed to setup test lock: %v", err)
			}

			config := &EnvironmentConfig{}
			got := config.Locked(tempDir)
			if got != tt.want {
				t.Errorf("EnvironmentConfig.Locked() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		func() bool {
			for i := 1; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}
