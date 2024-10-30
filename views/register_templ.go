// Code generated by templ - DO NOT EDIT.

// templ: version: v0.2.793
package views

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import templruntime "github.com/a-h/templ/runtime"

func Register() templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var1 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var1 == nil {
			templ_7745c5c3_Var1 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("<nav aria-label=\"Breadcrumb\"><ul class=\"uk-breadcrumb\"><li><a href=\"/\" hx-get=\"/\" hx-target=\"#content\" hx-push-url=\"true\">Home</a></li><li><span>Register</span></li></ul></nav><h2 class=\"uk-heading-line uk-h2 uk-card-title uk-text-center\"><span>Register</span></h2><div class=\"uk-width-1-3 uk-align-center\"><form hx-post=\"/register\" hx-redirect=\"/login\"><div class=\"uk-margin\"><div class=\"uk-inline uk-width-1-1\"><span class=\"uk-form-icon\" uk-icon=\"icon: user\"></span> <input class=\"uk-input\" type=\"text\" name=\"username\" placeholder=\"Username\" required aria-label=\"Not clickable icon\" required></div></div><div class=\"uk-margin\"><div class=\"uk-inline uk-width-1-1\"><span class=\"uk-form-icon uk-form-icon-flip\" uk-icon=\"icon: lock\"></span> <input class=\"uk-input\" type=\"password\" name=\"password\" placeholder=\"Password\" required aria-label=\"Not clickable icon\" required></div></div><div class=\"mt-4 uk-flex uk-flex-center\"><button type=\"submit\" class=\"uk-button uk-button-default mr-2\">Register</button> <a href=\"/login\" hx-get=\"/login\" hx-target=\"#content\" hx-push-url=\"true\" class=\"uk-button uk-button-default ml-2\">Back to login</a></div></form></div>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return templ_7745c5c3_Err
	})
}

var _ = templruntime.GeneratedTemplate
