// Package vault is the repository model: fields, secrets and their validation rules.
package vault

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrRepoNotEmpty        = errors.New("repository contains secrets; delete them first")
	ErrChangedOnDisk       = errors.New("repository file changed on disk since it was opened")
	ErrExists              = errors.New("already exists")
	ErrNotFound            = errors.New("not found")
	ErrRequiredWithSecrets = errors.New("cannot make a field required while the repository contains secrets")
)

const defaultPattern = ".+"

// Field describes one value tracked per secret.
type Field struct {
	Name     string `json:"name"`
	Required bool   `json:"required,omitempty"`
	Pattern  string `json:"pattern,omitempty"` // empty means defaultPattern
}

func (f Field) pattern() string {
	if f.Pattern == "" {
		return defaultPattern
	}
	return f.Pattern
}

// Repo is the decrypted content of a repository file.
type Repo struct {
	Fields  []Field                      `json:"fields"`
	Secrets map[string]map[string]string `json:"secrets"`
}

func (r *Repo) field(name string) (*Field, error) {
	for i := range r.Fields {
		if r.Fields[i].Name == name {
			return &r.Fields[i], nil
		}
	}
	return nil, fmt.Errorf("field %q: %w", name, ErrNotFound)
}

// AddField adds an optional field with the default pattern.
func (r *Repo) AddField(name string) error {
	if err := ValidateName(name); err != nil {
		return fmt.Errorf("field name: %w", err)
	}
	if _, err := r.field(name); err == nil {
		return fmt.Errorf("field %q: %w", name, ErrExists)
	}
	r.Fields = append(r.Fields, Field{Name: name})
	return nil
}

// DeleteField removes a field and any values stored for it.
func (r *Repo) DeleteField(name string) error {
	for i := range r.Fields {
		if r.Fields[i].Name == name {
			r.Fields = append(r.Fields[:i], r.Fields[i+1:]...)
			for _, vals := range r.Secrets {
				delete(vals, name)
			}
			return nil
		}
	}
	return fmt.Errorf("field %q: %w", name, ErrNotFound)
}

// AttribNames lists the valid field attribute names.
func AttribNames() []string { return []string{"required", "pattern"} }

// SetAttrib sets a field attribute, validating the value.
func (r *Repo) SetAttrib(field, attrib, value string) error {
	f, err := r.field(field)
	if err != nil {
		return err
	}
	switch attrib {
	case "required":
		if value != "true" && value != "false" {
			return fmt.Errorf(`required must be "true" or "false", got %q`, value)
		}
		if value == "true" && len(r.Secrets) > 0 {
			return ErrRequiredWithSecrets
		}
		f.Required = value == "true"
	case "pattern":
		if _, err := regexp.Compile(value); err != nil {
			return fmt.Errorf("invalid pattern: %w", err)
		}
		f.Pattern = value
	default:
		return fmt.Errorf("unknown attribute %q (valid: required, pattern)", attrib)
	}
	return nil
}

// GetAttrib returns a field attribute's effective value.
func (r *Repo) GetAttrib(field, attrib string) (string, error) {
	f, err := r.field(field)
	if err != nil {
		return "", err
	}
	switch attrib {
	case "required":
		return fmt.Sprint(f.Required), nil
	case "pattern":
		return f.pattern(), nil
	}
	return "", fmt.Errorf("unknown attribute %q (valid: required, pattern)", attrib)
}

// DeleteAttrib resets a field attribute to its default.
func (r *Repo) DeleteAttrib(field, attrib string) error {
	f, err := r.field(field)
	if err != nil {
		return err
	}
	switch attrib {
	case "required":
		f.Required = false
	case "pattern":
		f.Pattern = ""
	default:
		return fmt.Errorf("unknown attribute %q (valid: required, pattern)", attrib)
	}
	return nil
}

// ValidateValues checks values against the repository's fields.
func (r *Repo) ValidateValues(values map[string]string) error {
	for name := range values {
		if _, err := r.field(name); err != nil {
			return err
		}
	}
	for _, f := range r.Fields {
		v := values[f.Name]
		if v == "" {
			if f.Required {
				return fmt.Errorf("field %q is required", f.Name)
			}
			continue
		}
		re, err := regexp.Compile("(?s)^(?:" + f.pattern() + ")$")
		if err != nil {
			return fmt.Errorf("field %q has invalid pattern: %w", f.Name, err)
		}
		if !re.MatchString(v) {
			return fmt.Errorf("field %q does not match pattern %s", f.Name, f.pattern())
		}
	}
	return nil
}

// SetSecret creates or replaces a secret after validating its values.
func (r *Repo) SetSecret(name string, values map[string]string) error {
	if name == "" {
		return errors.New("secret name must not be empty")
	}
	if err := r.ValidateValues(values); err != nil {
		return err
	}
	if r.Secrets == nil {
		r.Secrets = map[string]map[string]string{}
	}
	cp := make(map[string]string, len(values))
	for k, v := range values {
		if v != "" {
			cp[k] = v
		}
	}
	r.Secrets[name] = cp
	return nil
}

// DeleteSecret removes a secret.
func (r *Repo) DeleteSecret(name string) error {
	if _, ok := r.Secrets[name]; !ok {
		return fmt.Errorf("secret %q: %w", name, ErrNotFound)
	}
	delete(r.Secrets, name)
	return nil
}
