package cmd

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	cmtconfig "github.com/cometbft/cometbft/config"
)

// parseAndApplyConfigChanges parses the config changes string and applies them to the nodeConfig
func parseAndApplyConfigChanges(nodeConfig *cmtconfig.Config, configChanges []string) error {
	if len(configChanges) == 0 {
		return nil
	}

	if err := applyConfigOverrides(nodeConfig, configChanges); err != nil {
		return err
	}

	return nil
}

// updateConfigField updates a field in the config based on dot notation
// Example: "consensus.timeout_propose=5s" or "log_level=debug" (for BaseConfig fields)
func updateConfigField(config *cmtconfig.Config, fieldPath, value string) error {
	parts := strings.Split(fieldPath, ".")

	configValue := reflect.ValueOf(config).Elem()

	// Handle BaseConfig fields (squashed/embedded fields)
	if len(parts) == 1 {
		// This might be a BaseConfig field, try to find it in the embedded struct
		baseConfigField := configValue.FieldByName("BaseConfig")
		if baseConfigField.IsValid() {
			targetFieldName := getFieldName(baseConfigField.Type(), parts[0])
			if targetFieldName != "" {
				targetField := baseConfigField.FieldByName(targetFieldName)
				if targetField.IsValid() && targetField.CanSet() {
					return setFieldValue(targetField, value)
				}
			}
		}

		// If not found in BaseConfig, try in the main Config struct
		targetFieldName := getFieldName(configValue.Type(), parts[0])
		if targetFieldName != "" {
			targetField := configValue.FieldByName(targetFieldName)
			if targetField.IsValid() && targetField.CanSet() {
				return setFieldValue(targetField, value)
			}
		}

		return fmt.Errorf("field not found: %s", parts[0])
	}

	// Handle nested fields (e.g., consensus.timeout_propose)
	current := configValue
	for i, part := range parts[:len(parts)-1] {
		field := current.FieldByName(getFieldName(current.Type(), part))
		if !field.IsValid() {
			return fmt.Errorf("field not found: %s", strings.Join(parts[:i+1], "."))
		}

		// If it's a pointer to a struct, get the element
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				// Initialize the pointer if it's nil
				field.Set(reflect.New(field.Type().Elem()))
			}
			field = field.Elem()
		}
		current = field
	}

	// Set the final field
	finalFieldName := parts[len(parts)-1]
	targetField := current.FieldByName(getFieldName(current.Type(), finalFieldName))
	if !targetField.IsValid() {
		return fmt.Errorf("field not found: %s", finalFieldName)
	}

	if !targetField.CanSet() {
		return fmt.Errorf("field cannot be set: %s", finalFieldName)
	}

	return setFieldValue(targetField, value)
}

// getFieldName finds the struct field name from mapstructure tag or field name
func getFieldName(structType reflect.Type, tagName string) string {
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)

		// Check mapstructure tag
		if tag := field.Tag.Get("mapstructure"); tag != "" {
			if tag == tagName {
				return field.Name
			}
		}

		// Check field name (case insensitive)
		if strings.EqualFold(field.Name, tagName) {
			return field.Name
		}
	}
	return ""
}

// setFieldValue sets the field value based on its type
func setFieldValue(field reflect.Value, value string) error {
	switch field.Type() {
	case reflect.TypeOf(time.Duration(0)):
		duration, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid duration format: %s", value)
		}
		field.Set(reflect.ValueOf(duration))

	case reflect.TypeOf(""):
		field.SetString(value)

	case reflect.TypeOf(true):
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean format: %s", value)
		}
		field.SetBool(boolVal)

	case reflect.TypeOf(int64(0)):
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid int64 format: %s", value)
		}
		field.SetInt(intVal)

	case reflect.TypeOf(int(0)):
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid int format: %s", value)
		}
		field.SetInt(int64(intVal))

	case reflect.TypeOf([]string{}):
		// Handle string slices - split by comma
		var slice []string
		if strings.TrimSpace(value) != "" {
			// Split by comma and trim whitespace
			parts := strings.Split(value, ",")
			for _, part := range parts {
				slice = append(slice, strings.TrimSpace(part))
			}
		}
		field.Set(reflect.ValueOf(slice))

	default:
		return fmt.Errorf("unsupported field type: %v", field.Type())
	}

	return nil
}

// parseConfigOverride parses a string like "consensus.timeout_propose=5s"
func parseConfigOverride(override string) (string, string, error) {
	parts := strings.SplitN(override, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid override format: %s (expected field=value)", override)
	}
	return parts[0], parts[1], nil
}

// applyConfigOverrides applies multiple overrides to the config
func applyConfigOverrides(config *cmtconfig.Config, overrides []string) error {
	for _, override := range overrides {
		fieldPath, value, err := parseConfigOverride(override)
		if err != nil {
			return err
		}

		if err := updateConfigField(config, fieldPath, value); err != nil {
			return fmt.Errorf("failed to set %s: %w", fieldPath, err)
		}
	}
	return nil
}
