package httpapi

import (
	"reflect"
	"testing"

	adminapi "xzji/internal/apigen/admin"
	pubapi "xzji/internal/apigen/public"
)

// 需要登录的接口表由契约生成，这里确认表里的名字都对应真实接口，且敏感接口没有漏掉。
func TestSecuredOperations(t *testing.T) {
	check := func(api string, secured map[string]bool, iface reflect.Type, mustSecure []string) {
		for op := range secured {
			if _, ok := iface.MethodByName(op); !ok {
				t.Errorf("%s：SecuredOperations 中的 %s 不是已定义的接口", api, op)
			}
		}
		for _, op := range mustSecure {
			if !secured[op] {
				t.Errorf("%s：%s 必须声明 security", api, op)
			}
		}
	}
	check("public", pubapi.SecuredOperations, reflect.TypeOf((*pubapi.StrictServerInterface)(nil)).Elem(),
		[]string{"GetMe", "UpdateMe", "ChangePassword"})
	check("admin", adminapi.SecuredOperations, reflect.TypeOf((*adminapi.StrictServerInterface)(nil)).Elem(),
		[]string{"GetMe", "ChangePassword", "GetSystemStatus", "ListUsers", "UpdateUser"})

	adminIface := reflect.TypeOf((*adminapi.StrictServerInterface)(nil)).Elem()
	open := map[string]bool{"GetCsrfToken": true, "GetSession": true, "Login": true, "Logout": true}
	for i := 0; i < adminIface.NumMethod(); i++ {
		name := adminIface.Method(i).Name
		if !open[name] && !adminapi.SecuredOperations[name] {
			t.Errorf("后台接口 %s 没有声明 security，后台除登录相关接口外都必须登录", name)
		}
	}
}
