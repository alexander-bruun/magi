package handlers

import (
	"github.com/alexander-bruun/magi/views"
	"github.com/dchest/captcha"
	fiber "github.com/gofiber/fiber/v2"
)

// CaptchaFormData represents form data for captcha verification
type CaptchaFormData struct {
	ID     string `json:"id"`
	Answer string `json:"answer"`
}

// HandleCaptchaPage serves the captcha verification page
func HandleCaptchaPage(c *fiber.Ctx) error {
	id := captcha.NewLen(6)
	errorMsg := ""
	if c.Query("error") == "invalid" {
		errorMsg = "Invalid captcha. Please try again."
	}

	return handleView(c, views.Captcha(errorMsg, id))
}

// HandleCaptchaImage serves captcha images
func HandleCaptchaImage(c *fiber.Ctx) error {
	c.Type("png")
	captcha.WriteImage(c.Response().BodyWriter(), c.Params("id"), 240, 80)
	return nil
}

// HandleCaptchaNew generates a new captcha ID
func HandleCaptchaNew(c *fiber.Ctx) error {
	id := captcha.NewLen(6)
	return c.JSON(fiber.Map{"captcha_id": id})
}

// HandleCaptchaVerify verifies captcha answers
func HandleCaptchaVerify(c *fiber.Ctx) error {
	var formData CaptchaFormData
	if err := c.BodyParser(&formData); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	id := formData.ID
	answer := formData.Answer
	if captcha.VerifyString(id, answer) {
		c.Cookie(&fiber.Cookie{
			Name:     "captcha_solved",
			Value:    "true",
			MaxAge:   3600, // 1 hour
			HTTPOnly: true,
			Secure:   isSecureRequest(c),
			SameSite: fiber.CookieSameSiteLaxMode,
		})
		// Redirect back to the original page
		redirectURL := c.Cookies("captcha_redirect")
		if redirectURL == "" {
			redirectURL = "/"
		}
		return c.Redirect(redirectURL, fiber.StatusSeeOther)
	}
	return c.Redirect("/captcha?error=invalid", fiber.StatusSeeOther)
}
