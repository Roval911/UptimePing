package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ProtoParser represents a parser for protobuf files
type ProtoParser struct {
	protoDir string
	services []*ServiceInfo
	messages []*MessageInfo
	enums    []*EnumInfo
}

// ServiceInfo contains information about a gRPC service
type ServiceInfo struct {
	Name    string
	Package string
	File    string
	Methods []*MethodInfo
	Options map[string]string
}

// MethodInfo contains information about a service method
type MethodInfo struct {
	Name            string
	InputType       string
	OutputType      string
	ClientStreaming bool
	ServerStreaming bool
	Options         map[string]string
}

// MessageInfo contains information about a message
type MessageInfo struct {
	Name    string
	Package string
	File    string
	Fields  []*FieldInfo
	Options map[string]string
}

// FieldInfo contains information about a message field
type FieldInfo struct {
	Name    string
	Type    string
	Number  int32
	Label   string
	Options map[string]string
}

// EnumInfo contains information about an enum
type EnumInfo struct {
	Name    string
	Package string
	File    string
	Values  []*EnumValueInfo
	Options map[string]string
}

// EnumValueInfo contains information about an enum value
type EnumValueInfo struct {
	Name    string
	Number  int32
	Options map[string]string
}

// NewProtoParser creates a new ProtoParser instance
func NewProtoParser(protoDir string) *ProtoParser {
	return &ProtoParser{
		protoDir: protoDir,
		services: []*ServiceInfo{},
		messages: []*MessageInfo{},
		enums:    []*EnumInfo{},
	}
}

// LoadAndValidateProtoFiles loads and validates proto files
func (p *ProtoParser) LoadAndValidateProtoFiles() error {
	// Check if directory exists
	if _, err := os.Stat(p.protoDir); os.IsNotExist(err) {
		return fmt.Errorf("proto directory does not exist: %s", p.protoDir)
	}

	// Recursively find all .proto files
	var protoFiles []string
	err := filepath.Walk(p.protoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".proto") {
			protoFiles = append(protoFiles, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk proto directory: %w", err)
	}

	if len(protoFiles) == 0 {
		return fmt.Errorf("no .proto files found in directory: %s", p.protoDir)
	}

	// Parse each proto file
	for _, protoFile := range protoFiles {
		if err := p.parseProtoFile(protoFile); err != nil {
			return fmt.Errorf("failed to parse proto file %s: %w", protoFile, err)
		}
	}

	return nil
}

// parseProtoFile parses a single proto file
func (p *ProtoParser) parseProtoFile(protoFile string) error {
	// Read proto file
	content, err := os.ReadFile(protoFile)
	if err != nil {
		return fmt.Errorf("failed to read proto file: %w", err)
	}

	// Get relative path
	relativePath, err := filepath.Rel(p.protoDir, protoFile)
	if err != nil {
		relativePath = protoFile
	}

	// Extract package name
	packageName := extractPackageName(string(content))

	// Add file info
	p.addFileInfo(relativePath, packageName, string(content))

	return nil
}

// extractPackageName extracts package name from proto content
func extractPackageName(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				packageName := parts[1]
				// Remove semicolon
				return strings.TrimSuffix(packageName, ";")
			}
		}
	}
	return "default"
}

// addFileInfo adds file information and extracts services, messages, and enums
func (p *ProtoParser) addFileInfo(file, packageName, content string) {
	// Extract services
	p.extractServices(file, packageName, content)

	// Extract messages
	p.extractMessages(file, packageName, content)

	// Extract enums
	p.extractEnums(file, packageName, content)
}

// extractServices extracts services from proto content
func (p *ProtoParser) extractServices(file, packageName, content string) {
	lines := strings.Split(content, "\n")
	inService := false
	var currentService *ServiceInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Start of service
		if strings.HasPrefix(line, "service ") {
			inService = true
			serviceName := extractServiceName(line)
			currentService = &ServiceInfo{
				Name:    serviceName,
				Package: packageName,
				File:    file,
				Methods: []*MethodInfo{},
				Options: make(map[string]string),
			}
			continue
		}

		// End of service
		if inService && line == "}" {
			if currentService != nil {
				p.services = append(p.services, currentService)
			}
			inService = false
			currentService = nil
			continue
		}

		// Service method
		if inService && strings.Contains(line, "rpc ") {
			if currentService != nil {
				method := extractMethodInfo(line)
				if method != nil {
					currentService.Methods = append(currentService.Methods, method)
				}
			}
		}
	}
}

// extractMessages extracts messages from proto content
func (p *ProtoParser) extractMessages(file, packageName, content string) {
	lines := strings.Split(content, "\n")
	inMessage := false
	var currentMessage *MessageInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Start of message
		if strings.HasPrefix(line, "message ") {
			inMessage = true
			messageName := extractMessageName(line)
			currentMessage = &MessageInfo{
				Name:    messageName,
				Package: packageName,
				File:    file,
				Fields:  []*FieldInfo{},
				Options: make(map[string]string),
			}
			continue
		}

		// End of message
		if inMessage && line == "}" {
			if currentMessage != nil {
				p.messages = append(p.messages, currentMessage)
			}
			inMessage = false
			currentMessage = nil
			continue
		}

		// Message field (proto3 doesn't use optional/required)
		if inMessage && strings.Contains(line, "=") && !strings.HasPrefix(line, "//") {
			if currentMessage != nil {
				field := extractFieldInfo(line)
				if field != nil && field.Name != "" && field.Type != "" {
					currentMessage.Fields = append(currentMessage.Fields, field)
				}
			}
		}
	}
}

// extractEnums extracts enums from proto content
func (p *ProtoParser) extractEnums(file, packageName, content string) {
	lines := strings.Split(content, "\n")
	inEnum := false
	var currentEnum *EnumInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Start of enum
		if strings.HasPrefix(line, "enum ") {
			inEnum = true
			enumName := extractEnumName(line)
			currentEnum = &EnumInfo{
				Name:    enumName,
				Package: packageName,
				File:    file,
				Values:  []*EnumValueInfo{},
				Options: make(map[string]string),
			}
			continue
		}

		// End of enum
		if inEnum && line == "}" {
			if currentEnum != nil {
				p.enums = append(p.enums, currentEnum)
			}
			inEnum = false
			currentEnum = nil
			continue
		}

		// Enum value
		if inEnum && strings.Contains(line, "=") {
			if currentEnum != nil {
				value := extractEnumValueInfo(line)
				if value != nil {
					currentEnum.Values = append(currentEnum.Values, value)
				}
			}
		}
	}
}

// Helper functions for extracting information

func extractServiceName(line string) string {
	// Must start with "service" keyword
	parts := strings.Fields(line)
	if len(parts) < 2 || parts[0] != "service" {
		return ""
	}

	name := parts[1]
	// Remove possible brace
	name = strings.TrimSuffix(name, "{")
	// Remove any trailing characters
	name = strings.TrimSpace(name)
	// Check if it looks like a valid identifier
	if len(name) > 0 && (name[0] == '_' || (name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z')) {
		return name
	}
	return ""
}

func extractMethodInfo(line string) *MethodInfo {
	// Example: rpc GetUser(GetUserRequest) returns (GetUserResponse) {}
	if !strings.Contains(line, "rpc ") {
		return nil
	}

	method := &MethodInfo{
		Options: make(map[string]string),
	}

	// Extract method name - first word after "rpc"
	rpcIndex := strings.Index(line, "rpc ")
	if rpcIndex == -1 {
		return nil
	}

	remaining := line[rpcIndex+4:] // After "rpc "

	// Find end of method name (space or '(')
	endIdx := strings.IndexAny(remaining, " (")
	if endIdx == -1 {
		return nil
	}

	method.Name = strings.TrimSpace(remaining[:endIdx])

	// Extract input type
	inputStart := strings.Index(remaining, "(")
	if inputStart != -1 {
		inputStart++
		if inputEnd := strings.Index(remaining[inputStart:], ")"); inputEnd != -1 {
			inputType := remaining[inputStart : inputStart+inputEnd]
			// Remove "stream " prefix if present
			inputType = strings.TrimPrefix(inputType, "stream ")
			method.InputType = strings.TrimSpace(inputType)
		}
	}

	// Extract output type
	returnsIndex := strings.Index(remaining, "returns")
	if returnsIndex != -1 {
		outputStart := strings.Index(remaining[returnsIndex:], "(")
		if outputStart != -1 {
			outputStart += returnsIndex
			if outputEnd := strings.Index(remaining[outputStart:], ")"); outputEnd != -1 {
				outputType := remaining[outputStart+1 : outputStart+outputEnd]
				// Remove "stream " prefix if present
				outputType = strings.TrimPrefix(outputType, "stream ")
				method.OutputType = strings.TrimSpace(outputType)
			}
		}
	}

	// Check for streaming
	method.ClientStreaming = strings.Contains(line, "stream ") &&
		strings.Index(line, "stream ") < strings.Index(line, "returns")
	method.ServerStreaming = strings.Contains(line, "stream ") &&
		strings.Index(line, "stream ") > strings.Index(line, "returns")

	return method
}

func extractMessageName(line string) string {
	// Must start with "message" keyword
	parts := strings.Fields(line)
	if len(parts) < 2 || parts[0] != "message" {
		return ""
	}

	name := parts[1]
	// Remove possible brace
	name = strings.TrimSuffix(name, "{")
	// Remove any trailing characters
	name = strings.TrimSpace(name)
	// Check if it looks like a valid identifier
	if len(name) > 0 && (name[0] == '_' || (name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z')) {
		return name
	}
	return ""
}

func extractFieldInfo(line string) *FieldInfo {
	field := &FieldInfo{
		Options: make(map[string]string),
	}

	// Determine label (proto3 defaults to optional)
	if strings.Contains(line, "optional ") {
		field.Label = "optional"
	} else if strings.Contains(line, "required ") {
		field.Label = "required"
	} else if strings.Contains(line, "repeated ") {
		field.Label = "repeated"
	} else {
		field.Label = "optional" // proto3 default
	}

	// Extract field number
	if idx := strings.Index(line, "="); idx != -1 {
		numStr := ""
		for i := idx + 1; i < len(line) && line[i] >= '0' && line[i] <= '9'; i++ {
			numStr += string(line[i])
		}
		if numStr != "" {
			// Convert string to int32
			if num, err := strconv.Atoi(numStr); err == nil {
				field.Number = int32(num)
			}
		}
	}

	// Extract name and type - proto3 format: type name = number;
	parts := strings.Fields(line)
	if len(parts) >= 3 && strings.Contains(line, "=") {
		// Example: string user_id = 1;
		field.Type = parts[0]
		field.Name = parts[1]
	} else if len(parts) >= 4 && strings.Contains(line, "=") {
		// Example: repeated string tags = 2;
		for i, part := range parts {
			if part == "optional" || part == "required" || part == "repeated" {
				if i+1 < len(parts) {
					field.Type = parts[i+1]
					if i+2 < len(parts) && parts[i+2] != "=" {
						field.Name = parts[i+2]
					}
				}
				break
			}
		}
	}

	return field
}

func extractEnumName(line string) string {
	// Must start with "enum" keyword
	parts := strings.Fields(line)
	if len(parts) < 2 || parts[0] != "enum" {
		return ""
	}

	name := parts[1]
	// Remove possible brace
	name = strings.TrimSuffix(name, "{")
	// Remove any trailing characters
	name = strings.TrimSpace(name)
	// Check if it looks like a valid identifier
	if len(name) > 0 && (name[0] == '_' || (name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z')) {
		return name
	}
	return ""
}

func extractEnumValueInfo(line string) *EnumValueInfo {
	value := &EnumValueInfo{
		Options: make(map[string]string),
	}

	// Example: UNKNOWN = 0;
	parts := strings.Split(line, "=")
	if len(parts) >= 2 {
		value.Name = strings.TrimSpace(parts[0])
		numStr := strings.TrimSpace(parts[1])
		// Remove semicolon
		numStr = strings.TrimSuffix(numStr, ";")
		// Convert string to int32
		if num, err := strconv.Atoi(numStr); err == nil {
			value.Number = int32(num)
		}
	}

	return value
}

// ParseProtoContent parses proto content from string and returns services
func (p *ProtoParser) ParseProtoContent(content string) ([]*ServiceInfo, error) {
	// Create temporary file for parsing
	tempFile := filepath.Join(p.protoDir, "temp.proto")
	err := os.WriteFile(tempFile, []byte(content), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	defer os.Remove(tempFile)

	// Parse the temporary file
	err = p.parseProtoFile(tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proto content: %w", err)
	}

	return p.services, nil
}

// GetServices returns list of all found services
func (p *ProtoParser) GetServices() []*ServiceInfo {
	return p.services
}

// GetMessages returns list of all found messages
func (p *ProtoParser) GetMessages() []*MessageInfo {
	return p.messages
}

// GetEnums returns list of all found enums
func (p *ProtoParser) GetEnums() []*EnumInfo {
	return p.enums
}

// GetServiceByName returns service by name
func (p *ProtoParser) GetServiceByName(name string) *ServiceInfo {
	for _, service := range p.services {
		if service.Name == name {
			return service
		}
	}
	return nil
}

// GetMessageByName returns message by name
func (p *ProtoParser) GetMessageByName(name string) *MessageInfo {
	for _, message := range p.messages {
		if message.Name == name {
			return message
		}
	}
	return nil
}

// GetEnumByName returns enum by name
func (p *ProtoParser) GetEnumByName(name string) *EnumInfo {
	for _, enum := range p.enums {
		if enum.Name == name {
			return enum
		}
	}
	return nil
}

// Validate validates extracted data
func (p *ProtoParser) Validate() error {
	// Check for duplicate service names
	serviceNames := make(map[string]bool)
	for _, service := range p.services {
		if serviceNames[service.Name] {
			return fmt.Errorf("duplicate service name: %s", service.Name)
		}
		serviceNames[service.Name] = true
	}

	// Check for duplicate message names
	messageNames := make(map[string]bool)
	for _, message := range p.messages {
		if messageNames[message.Name] {
			return fmt.Errorf("duplicate message name: %s", message.Name)
		}
		messageNames[message.Name] = true
	}

	// Check type references in methods
	for _, service := range p.services {
		for _, method := range service.Methods {
			if method.InputType != "" && !messageNames[method.InputType] {
				// Could be an enum
				if !p.isEnumType(method.InputType) {
					return fmt.Errorf("unknown input type %s in method %s", method.InputType, method.Name)
				}
			}
			if method.OutputType != "" && !messageNames[method.OutputType] {
				if !p.isEnumType(method.OutputType) {
					return fmt.Errorf("unknown output type %s in method %s", method.OutputType, method.Name)
				}
			}
		}
	}

	return nil
}

// isEnumType checks if type is an enum
func (p *ProtoParser) isEnumType(typeName string) bool {
	for _, enum := range p.enums {
		if enum.Name == typeName {
			return true
		}
	}
	return false
}

// PrintSummary prints summary information about found elements
func (p *ProtoParser) PrintSummary() {
	fmt.Printf("Proto Parser Summary:\n")
	fmt.Printf("Services: %d\n", len(p.services))
	fmt.Printf("Messages: %d\n", len(p.messages))
	fmt.Printf("Enums: %d\n", len(p.enums))

	fmt.Printf("\nServices:\n")
	for _, service := range p.services {
		fmt.Printf("  - %s (%s) - %d methods\n", service.Name, service.Package, len(service.Methods))
	}

	fmt.Printf("\nMessages:\n")
	for _, message := range p.messages {
		fmt.Printf("  - %s (%s) - %d fields\n", message.Name, message.Package, len(message.Fields))
	}

	fmt.Printf("\nEnums:\n")
	for _, enum := range p.enums {
		fmt.Printf("  - %s (%s) - %d values\n", enum.Name, enum.Package, len(enum.Values))
	}
}
