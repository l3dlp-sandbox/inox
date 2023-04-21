package internal

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	PERMISSION_KINDS = []struct {
		PermissionKind PermissionKind
		Name           string
	}{
		{ReadPerm, "read"},
		{WritePerm, "write"},
		{DeletePerm, "delete"},
		{UsePerm, "use"},
		{ConsumePerm, "consume"},
		{ProvidePerm, "provide"},
		{SeePerm, "see"},

		{UpdatePerm, "update"},
		{CreatePerm, "create"},
		{WriteStreamPerm, "write-stream"},
	}

	PERMISSION_KIND_NAMES = utils.MapSlice(PERMISSION_KINDS, func(e struct {
		PermissionKind PermissionKind
		Name           string
	}) string {
		return e.Name
	})
)

/*
	read
	write/update
	write/write-stream
	write/append
*/

type PermissionKind int

const (
	ReadPerm    PermissionKind = (1 << iota)
	WritePerm                  = (1 << iota)
	DeletePerm                 = (1 << iota)
	UsePerm                    = (1 << iota)
	ConsumePerm                = (1 << iota)
	ProvidePerm                = (1 << iota)
	SeePerm                    = (1 << iota)
	//up to 16 major permission kinds

	UpdatePerm      = WritePerm + (1 << 16)
	CreatePerm      = WritePerm + (2 << 16)
	WriteStreamPerm = WritePerm + (4 << 16)
	//up to 16 minor permission kinds for each major one
)

func (k PermissionKind) Major() PermissionKind {
	return k & 0xff
}

func (k PermissionKind) IsMajor() bool {
	return k == (k & 0xff)
}

func (k PermissionKind) IsMinor() bool {
	return k != (k & 0xff)
}

func (k PermissionKind) Includes(otherKind PermissionKind) bool {
	return k.Major() == otherKind.Major() && ((k.IsMajor() && otherKind.IsMinor()) || k == otherKind)
}

func (kind PermissionKind) String() string {
	if kind < 0 {
		return "<invalid permission kind>"
	}

	for _, e := range PERMISSION_KINDS {
		if e.PermissionKind == kind {
			return e.Name
		}
	}

	return "<invalid permission kind>"
}

func PermissionKindFromString(s string) (PermissionKind, bool) {
	for _, e := range PERMISSION_KINDS {
		if e.Name == s {
			return e.PermissionKind, true
		}
	}

	return 0, false
}

func isPermissionKindName(s string) bool {
	_, ok := PermissionKindFromString(s)
	return ok
}

// A Permission carries a kind and can include narrower permissions, for example
// (read http://**) includes (read https://example.com).
type Permission interface {
	Kind() PermissionKind
	Includes(Permission) bool
	String() string
}

type NotAllowedError struct {
	Permission Permission
	Message    string
}

func NewNotAllowedError(perm Permission) NotAllowedError {
	return NotAllowedError{
		Permission: perm,
		Message:    fmt.Sprintf("not allowed, missing permission: %s", perm.String()),
	}
}

func (err NotAllowedError) Error() string {
	return err.Message
}

func (err NotAllowedError) Is(target error) bool {
	notAllowedErr, ok := target.(NotAllowedError)
	if !ok {
		return false
	}

	return err.Permission.Includes(notAllowedErr.Permission) && notAllowedErr.Permission.Includes(err.Permission) &&
		err.Message == notAllowedErr.Message
}

type GlobalVarPermission struct {
	Kind_ PermissionKind
	Name  string //"*" means any
}

func (perm GlobalVarPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm GlobalVarPermission) Includes(otherPerm Permission) bool {
	otherGlobVarPerm, ok := otherPerm.(GlobalVarPermission)
	if !ok || !perm.Kind().Includes(otherGlobVarPerm.Kind()) {
		return false
	}

	return perm.Name == "*" || perm.Name == otherGlobVarPerm.Name
}

func (perm GlobalVarPermission) String() string {
	return fmt.Sprintf("[%s global(s) '%s']", perm.Kind_, perm.Name)
}

//

type EnvVarPermission struct {
	Kind_ PermissionKind
	Name  string //"*" means any
}

func (perm EnvVarPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm EnvVarPermission) Includes(otherPerm Permission) bool {
	otherEnvVarPerm, ok := otherPerm.(EnvVarPermission)
	if !ok || !perm.Kind().Includes(otherEnvVarPerm.Kind()) {
		return false
	}

	return perm.Name == "*" || perm.Name == otherEnvVarPerm.Name
}

func (perm EnvVarPermission) String() string {
	return fmt.Sprintf("[%s env '%s']", perm.Kind_, perm.Name)
}

//

type RoutinePermission struct {
	Kind_ PermissionKind
}

func (perm RoutinePermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm RoutinePermission) Includes(otherPerm Permission) bool {
	otherRoutinePerm, ok := otherPerm.(RoutinePermission)

	return ok && perm.Kind_.Includes(otherRoutinePerm.Kind_)
}

func (perm RoutinePermission) String() string {
	return fmt.Sprintf("[%s routine]", perm.Kind_)
}

type FilesystemPermission struct {
	Kind_  PermissionKind
	Entity WrappedString //Path, PathPattern ...
}

func CreateFsReadPerm(entity WrappedString) FilesystemPermission {
	return FilesystemPermission{Kind_: ReadPerm, Entity: entity}
}

func (perm FilesystemPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm FilesystemPermission) Includes(otherPerm Permission) bool {
	otherFsPerm, ok := otherPerm.(FilesystemPermission)
	if !ok || !perm.Kind_.Includes(otherFsPerm.Kind_) {
		return false
	}

	switch e := perm.Entity.(type) {
	case Path:
		otherPath, ok := otherFsPerm.Entity.(Path)
		return ok && e == otherPath
	case PathPattern:
		return e.Test(nil, otherFsPerm.Entity)
	}

	return false
}

func (perm FilesystemPermission) String() string {
	return fmt.Sprintf("[%s path(s) %s]", perm.Kind_, perm.Entity)
}

type CommandPermission struct {
	CommandName         WrappedString //string or Path or PathPattern
	SubcommandNameChain []string      //can be empty
}

func (perm CommandPermission) Kind() PermissionKind {
	return UsePerm
}

func (perm CommandPermission) Includes(otherPerm Permission) bool {

	otherCmdPerm, ok := otherPerm.(CommandPermission)
	if !ok || !perm.Kind().Includes(otherCmdPerm.Kind()) {
		return false
	}

	switch cmdName := perm.CommandName.(type) {
	case Str:
		otherCommandName, ok := otherCmdPerm.CommandName.(Str)
		if !ok || otherCommandName != cmdName {
			return false
		}
	case Path:
		otherCommandName, ok := otherCmdPerm.CommandName.(Path)
		if !ok || otherCommandName != cmdName {
			return false
		}
	case PathPattern:
		switch otherCmdPerm.CommandName.(type) {
		case Path, PathPattern:
			if !cmdName.Test(nil, otherCmdPerm.CommandName) {
				return false
			}
		default:
			return false
		}
	default:
		return false
	}

	if len(otherCmdPerm.SubcommandNameChain) != len(perm.SubcommandNameChain) {
		return false
	}

	for i, name := range perm.SubcommandNameChain {
		if otherCmdPerm.SubcommandNameChain[i] != name {
			return false
		}
	}

	return true
}

func (perm CommandPermission) String() string {
	b := bytes.NewBufferString("[exec command:")
	b.WriteString(fmt.Sprint(perm.CommandName))

	if len(perm.SubcommandNameChain) == 0 {
		b.WriteString(" <no subcommand>")
	}

	for _, name := range perm.SubcommandNameChain {
		b.WriteString(" ")
		b.WriteString(name)
	}
	b.WriteString("]")

	return b.String()
}

type HttpPermission struct {
	Kind_  PermissionKind
	Entity WrappedString //URL, URLPattern, HTTPHost, HTTPHostPattern ....
}

func (perm HttpPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm HttpPermission) Includes(otherPerm Permission) bool {
	otherHttpPerm, ok := otherPerm.(HttpPermission)
	if !ok || !perm.Kind_.Includes(otherHttpPerm.Kind_) {
		return false
	}

	switch e := perm.Entity.(type) {
	case URL:
		otherURL, ok := otherHttpPerm.Entity.(URL)
		parsedURL, _ := url.Parse(string(e))

		if parsedURL.RawQuery == "" {
			parsedURL.ForceQuery = false

			otherParsedURL, _ := url.Parse(string(otherURL))
			otherParsedURL.RawQuery = ""
			otherParsedURL.ForceQuery = false

			return parsedURL.String() == otherParsedURL.String()
		}

		return ok && e == otherURL
	case URLPattern:
		otherURLPattern, ok := otherHttpPerm.Entity.(URLPattern)
		if ok && e.IsPrefixPattern() && otherURLPattern.IsPrefixPattern() &&
			strings.HasPrefix(strings.ReplaceAll(string(e), "/...", "/"), strings.ReplaceAll(string(otherURLPattern), "/...", "/")) {
			return true
		}

		return e.Test(nil, otherHttpPerm.Entity)
	case Host:
		host := e.WithoutScheme()
		switch other := otherHttpPerm.Entity.(type) {
		case URL:
			otherURL, err := url.Parse(string(other))
			if err != nil {
				return false
			}
			return otherURL.Scheme == string(e.Scheme()) && otherURL.Host == host
		case URLPattern:
			otherURL, err := url.Parse(string(other))
			if err != nil {
				return false
			}
			return otherURL.Scheme == string(e.Scheme()) && otherURL.Host == host
		case Host:
			return e == other
		}
	case HostPattern:
		return e.Test(nil, otherHttpPerm.Entity)
	}

	return false
}

func (perm HttpPermission) String() string {
	return fmt.Sprintf("[%s %s]", perm.Kind_, perm.Entity)
}

type WebsocketPermission struct {
	Kind_    PermissionKind
	Endpoint ResourceName
}

func (perm WebsocketPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm WebsocketPermission) String() string {
	return fmt.Sprintf("[websocket %s %s]", perm.Kind_, perm.Endpoint)
}

func (perm WebsocketPermission) Includes(otherPerm Permission) bool {
	otherWsPerm, ok := otherPerm.(WebsocketPermission)
	if !ok || !perm.Kind_.Includes(otherWsPerm.Kind_) {
		return false
	}

	return perm.Kind_ == ProvidePerm || perm.Endpoint == otherWsPerm.Endpoint
}

type DNSPermission struct {
	Kind_  PermissionKind
	Domain WrappedString //Host | HostPattern
}

func (perm DNSPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm DNSPermission) String() string {
	return fmt.Sprintf("[dns %s %s]", perm.Kind_, perm.Domain)
}

func (perm DNSPermission) Includes(otherPerm Permission) bool {
	otherDnsPerm, ok := otherPerm.(DNSPermission)
	if !ok || !perm.Kind_.Includes(otherDnsPerm.Kind_) {
		return false
	}

	switch domain := perm.Domain.(type) {
	case HostPattern:
		switch otherDomain := otherDnsPerm.Domain.(type) {
		case Host:
			return domain.Test(nil, otherDomain)
		case HostPattern:
			return domain.includesPattern(otherDomain)
		}
	case Host:
		switch otherDomain := otherDnsPerm.Domain.(type) {
		case Host:
			return domain == otherDomain
		case HostPattern:
			return false
		}
	}

	return false

}

//----------------------------------------------------------------------

type RawTcpPermission struct {
	Kind_  PermissionKind
	Domain WrappedString //Host | HostPattern
}

func (perm RawTcpPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm RawTcpPermission) String() string {
	return fmt.Sprintf("[tcp %s %s]", perm.Kind_, perm.Domain)
}

func (perm RawTcpPermission) Includes(otherPerm Permission) bool {
	otherTcpPerm, ok := otherPerm.(RawTcpPermission)
	if !ok || !perm.Kind().Includes(otherTcpPerm.Kind_) {
		return false
	}

	switch domain := perm.Domain.(type) {
	case HostPattern:
		switch otherDomain := otherTcpPerm.Domain.(type) {
		case Host:
			return domain.Test(nil, otherDomain)
		case HostPattern:
			return domain.includesPattern(otherDomain)
		}
	case Host:
		switch otherDomain := otherTcpPerm.Domain.(type) {
		case Host:
			return domain == otherDomain
		case HostPattern:
			return false
		}
	}

	return false

}

type ValueVisibilityPermission struct {
	Pattern Pattern
}

func (perm ValueVisibilityPermission) Kind() PermissionKind {
	return SeePerm
}

func (perm ValueVisibilityPermission) String() string {
	return fmt.Sprintf("[see value matching %s]", perm.Pattern)
}

func (perm ValueVisibilityPermission) Includes(otherPerm Permission) bool {
	otherVisibilityPerm, ok := otherPerm.(ValueVisibilityPermission)
	if !ok || !perm.Kind().Includes(otherPerm.Kind()) {
		return false
	}

	//TODO: support all patterns

	if exact, ok := otherVisibilityPerm.Pattern.(*ExactValuePattern); ok {
		return perm.Pattern.Test(nil, exact.value)
	}

	return perm.Pattern.Equal(nil, otherVisibilityPerm.Pattern, map[uintptr]uintptr{}, 0)
}

type SystemGraphAccessPermission struct {
	Kind_ PermissionKind
}

func (perm SystemGraphAccessPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm SystemGraphAccessPermission) String() string {
	return fmt.Sprintf("[%s system graph]", perm.Kind_.String())
}

func (perm SystemGraphAccessPermission) Includes(otherPerm Permission) bool {
	otherSysGraphPerm, ok := otherPerm.(SystemGraphAccessPermission)
	return ok && perm.Kind_.Includes(otherSysGraphPerm.Kind_)
}
