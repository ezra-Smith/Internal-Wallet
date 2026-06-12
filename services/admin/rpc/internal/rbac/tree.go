package rbac

import (
	"strings"

	"google.golang.org/protobuf/types/known/structpb"
)

// BuildPermissionsTree converts permission codes into a nested tree structure.
//
// Examples:
// - ["user.read","user.write"] => {"user":{"read":true,"write":true}}
// - ["rpc:ListAdmins","rpc:*"] => {"rpc":{"ListAdmins":true,"*":true}}
func BuildPermissionsTree(perms []string) (*structpb.Struct, error) {
	root := map[string]interface{}{}
	for _, p := range perms {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "*" {
			root["*"] = true
			continue
		}

		parts := strings.FieldsFunc(p, func(r rune) bool {
			return r == '.' || r == ':'
		})
		if len(parts) == 0 {
			continue
		}
		if len(parts) == 1 {
			root[parts[0]] = true
			continue
		}

		curr := root
		for i := 0; i < len(parts); i++ {
			key := parts[i]
			if i == len(parts)-1 {
				curr[key] = true
				continue
			}
			next, ok := curr[key].(map[string]interface{})
			if !ok {
				next = map[string]interface{}{}
				curr[key] = next
			}
			curr = next
		}
	}
	return structpb.NewStruct(root)
}
