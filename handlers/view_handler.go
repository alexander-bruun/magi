package handlers

import (
	"fmt"

	"github.com/a-h/templ"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

func HandleView(c *fiber.Ctx, content templ.Component) error {
	if c.Get("HX-Request") != "" {
		handler := adaptor.HTTPHandler(templ.Handler(content))
		return handler(c)
	}

	accessToken := c.Cookies("access_token")

	userRole := ""
	if accessToken != "" {
		claims, err := models.ValidateToken(accessToken)
		if err == nil && claims != nil {
			userName := claims["user_name"].(string)
			user, err := models.FindUserByUsername(userName)
			if err != nil {
				return fmt.Errorf("failed to find user: %s", userName)
			} else {
				userRole = user.Role
			}
		}
	}

	base := views.Layout(content, userRole)
	handler := adaptor.HTTPHandler(templ.Handler(base))
	return handler(c)
}

func HandleHome(c *fiber.Ctx) error {
	recentlyAdded, _, _ := models.SearchMangas("", 1, 10, "created_at", "desc", "", "")
	recentlyUpdated, _, _ := models.SearchMangas("", 1, 10, "updated_at", "desc", "", "")
	return HandleView(c, views.Home(recentlyAdded, recentlyUpdated))
}

func HandleNotFound(c *fiber.Ctx) error {
	return HandleView(c, views.NotFound())
}

func HandleAdmin(c *fiber.Ctx) error {
	libraries, _ := models.GetLibraries()
	return HandleView(c, views.Admin(libraries))
}
