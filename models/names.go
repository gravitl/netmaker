package models

import (
	"time"

	"github.com/goombaio/namegenerator"
)

var logoString = retrieveLogo()

// GenerateNodeName - generates a random node name
func GenerateNodeName() string {
	seed := time.Now().UTC().UnixNano()
	nameGenerator := namegenerator.NewNameGenerator(seed)
	return nameGenerator.Generate()
}

// RetrieveLogo - retrieves the ascii art logo for Netmaker
func RetrieveLogo() string {
	return logoString
}

// SetLogo - sets the logo ascii art
func SetLogo(logo string) {
	logoString = logo
}

func retrieveLogo() string {
	return `              
 __   __     ______     ______   __    __     ______     __  __     ______     ______    
/\ "-.\ \   /\  ___\   /\__  _\ /\ "-./  \   /\  __ \   /\ \/ /    /\  ___\   /\  == \   
\ \ \-.  \  \ \  __\   \/_/\ \/ \ \ \-./\ \  \ \  __ \  \ \  _"-.  \ \  __\   \ \  __<   
 \ \_\\"\_\  \ \_____\    \ \_\  \ \_\ \ \_\  \ \_\ \_\  \ \_\ \_\  \ \_____\  \ \_\ \_\ 
  \/_/ \/_/   \/_____/     \/_/   \/_/  \/_/   \/_/\/_/   \/_/\/_/   \/_____/   \/_/ /_/ 
                                                                                         																							 
`
}
