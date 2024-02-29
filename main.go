package main

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	ServerAddress  string                 `json:"server_address"`
	APIURL         string                 `json:"api_url"`
	APIKey         string                 `json:"api_key"`
	SystemPrompt   string                 `json:"system_prompt"`
	AuthToken      string                 `json:"auth_token"`
	DefaultModel   string                 `json:"default_model"`
	OllamaParams   map[string]interface{} `json:"ollama_params"`
	ResponseFields []string               `json:"response_fields"`
	RequestTimeout int                    `json:"request_timeout"`
	StripNewline   bool                   `json:"strip_newline"`
}

type TemplateConfig struct {
	Templates       map[string]*template.Template
	Params          map[string]map[string]interface{}
	Fields          map[string][]string
	RequestTimeouts map[string]int
}

type OllamaResponse struct {
	Model              string        `json:"model"`
	CreatedAt          string        `json:"created_at"`
	Response           string        `json:"response"`
	Done               bool          `json:"done"`
	Context            []interface{} `json:"context"`
	TotalDuration      int64         `json:"total_duration"`
	LoadDuration       int64         `json:"load_duration"`
	PromptEvalCount    int           `json:"prompt_eval_count"`
	PromptEvalDuration int64         `json:"prompt_eval_duration"`
	EvalCount          int           `json:"eval_count"`
	EvalDuration       int64         `json:"eval_duration"`
}

type TemplateData struct {
	Query string
}

func loadConfig(configPath string) (*Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func loadAndCacheTemplates(templatesDir string) (*TemplateConfig, error) {
	templateConfig := &TemplateConfig{Templates: make(map[string]*template.Template)}

	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		log.Printf("Templates directory '%s' does not exist, creating it...", templatesDir)
		if err := os.MkdirAll(templatesDir, os.ModePerm); err != nil {
			return nil, err
		}
	}

	files, err := os.ReadDir(templatesDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		templateName := file.Name()
		if filepath.Ext(templateName) == ".json" {
			templatePath := filepath.Join(templatesDir, templateName)
			templateString, err := os.ReadFile(templatePath)
			if err != nil {
				log.Printf("Failed to load template file %s: %v", templatePath, err)
				continue
			}

			tmpl, err := template.New(templateName).Parse(string(templateString))
			if err != nil {
				log.Printf("Failed to parse template %s: %v", templateName, err)
				continue
			}

			templateConfig.Templates[templateName[:len(templateName)-len(".json")]] = tmpl
		}
	}

	if len(templateConfig.Templates) == 0 {
		log.Println("No templates found, creating a default template...")
		defaultTemplateContent := `{{.Query}} Default template response.`
		tmpl, err := template.New("default").Parse(defaultTemplateContent)
		if err != nil {
			return nil, err
		}
		templateConfig.Templates["default"] = tmpl

		defaultTemplatePath := filepath.Join(templatesDir, "default.json")
		if err := os.WriteFile(defaultTemplatePath, []byte(defaultTemplateContent), os.ModePerm); err != nil {
			log.Printf("Failed to save default template to disk: %v", err)
		}
	}

	return templateConfig, nil
}

func authenticate(config *Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token != "Bearer "+config.AuthToken {
			log.Printf("Unauthorized access attempt from token ending in: '%s', from: %s", token[len(token)-1:], r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		log.Println("Successful authentication from:", r.RemoteAddr)
		next(w, r)
	}
}

func processTemplate(tmpl *template.Template, data TemplateData) (string, error) {
	var processedTemplate bytes.Buffer
	if err := tmpl.Execute(&processedTemplate, data); err != nil {
		return "", err
	}
	return processedTemplate.String(), nil
}

func templateHandler(config *Config, templateConfig *TemplateConfig, templateName string) http.HandlerFunc {
	return authenticate(config, func(w http.ResponseWriter, r *http.Request) {
		var haRequest map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&haRequest); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Extract 'query' directly to use as the 'prompt' in the Ollama request
		query, ok := haRequest["query"].(string)
		if !ok {
			http.Error(w, "Query parameter missing or not a string", http.StatusBadRequest)
			return
		}

		// Prepare the prompt using the template, if needed, or directly from the 'query'
		var fullPrompt string
		if tmpl, ok := templateConfig.Templates[templateName]; ok {
			templateData := TemplateData{Query: query}
			processedPrompt, err := processTemplate(tmpl, templateData)
			if err != nil {
				http.Error(w, "Template processing failed", http.StatusInternalServerError)
				return
			}
			fullPrompt = processedPrompt
		} else {
			fullPrompt = query // Use the query as the prompt directly if no template processing is required
		}

		// Ensure the model is correctly set from the config or request
		model := config.DefaultModel
		if modelFromRequest, ok := haRequest["model"].(string); ok && modelFromRequest != "" {
			model = modelFromRequest
		}

		// Prepare the Ollama request with corrected fields
		ollamaRequest := config.OllamaParams // Start with global Ollama parameters
		ollamaRequest["prompt"] = fullPrompt
		ollamaRequest["model"] = model // Ensure the model is correctly assigned

		requestBody, err := json.Marshal(ollamaRequest)
		if err != nil {
			log.Printf("Error marshaling Ollama request: %v", err)
			return
		}

		// Setup the HTTP request to Ollama API
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.RequestTimeout)*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.APIURL, bytes.NewBuffer(requestBody))
		if err != nil {
			log.Printf("Error creating request to Ollama API: %v", err)
			return
		}
		req.Header.Add("Authorization", "Bearer "+config.APIKey)
		req.Header.Add("Content-Type", "application/json")

		// Send the request to Ollama API
		// Send the request to Ollama API
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Failed to send request to Ollama API: %v", err)
			return
		}
		defer resp.Body.Close()

		// Read and unmarshal the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Failed to read response body: %v", err)
			return
		}

		var ollamaResponse OllamaResponse
		if err := json.Unmarshal(body, &ollamaResponse); err != nil {
			log.Printf("Error unmarshaling response from Ollama API: %v", err)
			return
		}

		// Create a filtered response based on what's needed
		filteredResponse := map[string]interface{}{
			"response": ollamaResponse.Response,
		}

		// If filteredResponse contains any of the fields from the config, add them
		// Convert ollamaResponse to a map
		ollamaResponseMap := make(map[string]interface{})
		err = json.Unmarshal(body, &ollamaResponseMap)
		if err != nil {
			log.Printf("Error unmarshaling response from Ollama API: %v", err)
			return
		}

		for _, field := range config.ResponseFields {
			if value, ok := ollamaResponseMap[field]; ok {
				filteredResponse[field] = value
			}
		}

		// If the config has strip_newline set to true, remove newlines
		if config.StripNewline {
			filteredResponse["response"] = strings.ReplaceAll(ollamaResponse.Response, "\n", " ")
		}

		// Send the filtered response back to the client
		responseBody, err := json.Marshal(filteredResponse)
		if err != nil {
			log.Printf("Error marshaling filtered response: %v", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(responseBody)
	})
}

func main() {
	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load server configuration: %v", err)
	}

	templateConfig, err := loadAndCacheTemplates("./templates")
	if err != nil {
		log.Fatalf("Failed to load and cache templates: %v", err)
	}

	for templateName := range templateConfig.Templates {
		http.HandleFunc("/template/"+templateName, templateHandler(config, templateConfig, templateName))
		println("-  /template/" + templateName)
	}

	log.Println("Starting server on", config.ServerAddress)
	if err := http.ListenAndServe(config.ServerAddress, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
