package controllers

import (
	"io/ioutil"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pushpaldev/base-api/config"
	"github.com/pushpaldev/base-api/helpers"
	"github.com/pushpaldev/base-api/helpers/params"
	"github.com/pushpaldev/base-api/models"
	"github.com/pushpaldev/base-api/services"
	"github.com/pushpaldev/base-api/store"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/gin-gonic/gin.v1"
)

type AuthController struct {
}

func NewAuthController() AuthController {
	return AuthController{}
}

func (ac AuthController) Authentication(c *gin.Context) {
	privateKeyFile, _ := ioutil.ReadFile(config.GetString(c, "rsa_private"))
	privateKey, _ := jwt.ParseRSAPrivateKeyFromPEM(privateKeyFile)

	userInput := models.User{}
	if err := c.Bind(&userInput); err != nil {
		c.AbortWithError(http.StatusBadRequest, helpers.ErrorWithCode("invalid_input", "Failed to bind the body data"))
		return
	}

	user, err := store.FindUser(c, params.M{"email": userInput.Email})
	if err != nil {
		c.AbortWithError(http.StatusNotFound, helpers.ErrorWithCode("user_does_not_exist", "User does not exist"))
		return
	}

	if !user.Active {
		c.AbortWithError(http.StatusNotFound, helpers.ErrorWithCode("user_needs_activation", "User needs to be activated"))
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(userInput.Password))
	if err != nil {
		c.AbortWithError(http.StatusUnauthorized, helpers.ErrorWithCode("incorrect_password", "Password is not correct"))
		return
	}

	apiToken, err := store.AddLoginToken(c, user, c.ClientIP())
	if err != nil {
		c.Error(err)
		c.Abort()
		return
	}

	token := jwt.New(jwt.GetSigningMethod(jwt.SigningMethodRS256.Alg()))
	claims := make(jwt.MapClaims)
	claims["iat"] = time.Now().Unix()
	claims["id"] = user.Id
	claims["token"] = apiToken.Id

	token.Claims = claims
	tokenString, err := token.SignedString(privateKey)

	services.GetRedis(c).InvalidateObject(user.Id)

	c.JSON(http.StatusOK, gin.H{"token": tokenString, "users": user.Sanitize()})
}

func (ac AuthController) LogOut(c *gin.Context) {
	if err := store.RemoveLoginToken(c); err != nil {
		c.Error(err)
		c.Abort()
		return
	}

	services.GetRedis(c).InvalidateObject(store.Current(c).Id)

	c.JSON(200, nil)
}
