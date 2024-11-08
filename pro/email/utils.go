package email

import "strings"

// mail related images hosted on github
var (
	netmakerLogoTeal = "https://raw.githubusercontent.com/gravitl/netmaker/netmaker_logos/img/logos/netmaker-logo-2.png"
)

type EmailBodyBuilder interface {
	WithHeadline(text string) EmailBodyBuilder
	WithParagraph(text string) EmailBodyBuilder
	WithHtml(text string) EmailBodyBuilder
	WithSignature() EmailBodyBuilder
	Build() string
}

type EmailBodyBuilderWithH1HeadlineAndImage struct {
	headline     string
	bodyContent  []string
	hasSignature bool
}

func (b *EmailBodyBuilderWithH1HeadlineAndImage) WithHeadline(text string) EmailBodyBuilder {
	b.headline = text
	return b
}

func (b *EmailBodyBuilderWithH1HeadlineAndImage) WithParagraph(text string) EmailBodyBuilder {
	b.bodyContent = append(b.bodyContent, styledParagraph(text))
	return b
}

func (b *EmailBodyBuilderWithH1HeadlineAndImage) WithHtml(text string) EmailBodyBuilder {
	b.bodyContent = append(b.bodyContent, text)
	return b
}

func (b *EmailBodyBuilderWithH1HeadlineAndImage) WithSignature() EmailBodyBuilder {
	b.hasSignature = true
	return b
}

func (b *EmailBodyBuilderWithH1HeadlineAndImage) Build() string {
	bodyContent := strings.Join(b.bodyContent, "")

	// TODO: Edit design to add signature.
	//signature := ""
	//if b.hasSignature {
	//	signature = styledSignature()
	//}

	return `
<!doctype html>
<html lang="en">
  <head>
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
    <title>Simple Transactional Email</title>
    <style media="all" type="text/css">
@media all {
  .btn-primary table td:hover {
    background-color: #ec0867 !important;
  }

  .btn-primary a:hover {
    background-color: #ec0867 !important;
    border-color: #ec0867 !important;
  }
}
@media only screen and (max-width: 640px) {
  .main p,
.main td,
.main span {
    font-size: 16px !important;
  }

  .wrapper {
    padding: 8px !important;
  }

  .content {
    padding: 0 !important;
  }

  .container {
    padding: 0 !important;
    padding-top: 8px !important;
    width: 100% !important;
  }

  .main {
    border-left-width: 0 !important;
    border-radius: 0 !important;
    border-right-width: 0 !important;
  }

  .btn table {
    max-width: 100% !important;
    width: 100% !important;
  }

  .btn a {
    font-size: 16px !important;
    max-width: 100% !important;
    width: 100% !important;
  }
}
@media all {
  .ExternalClass {
    width: 100%;
  }

  .ExternalClass,
.ExternalClass p,
.ExternalClass span,
.ExternalClass font,
.ExternalClass td,
.ExternalClass div {
    line-height: 100%;
  }

  .apple-link a {
    color: inherit !important;
    font-family: inherit !important;
    font-size: inherit !important;
    font-weight: inherit !important;
    line-height: inherit !important;
    text-decoration: none !important;
  }

  #MessageViewBody a {
    color: inherit;
    text-decoration: none;
    font-size: inherit;
    font-family: inherit;
    font-weight: inherit;
    line-height: inherit;
  }
}
</style>
  </head>
  <body style="font-family: Helvetica, sans-serif; -webkit-font-smoothing: antialiased; font-size: 16px; line-height: 1.3; -ms-text-size-adjust: 100%; -webkit-text-size-adjust: 100%; background-color: #f4f5f6; margin: 0; padding: 0;">
    <table role="presentation" border="0" cellpadding="0" cellspacing="0" class="body" style="border-collapse: separate; mso-table-lspace: 0pt; mso-table-rspace: 0pt; background-color: #f4f5f6; width: 100%;" width="100%" bgcolor="#f4f5f6">
      <tr>
        <td style="font-family: Helvetica, sans-serif; font-size: 16px; vertical-align: top;" valign="top">&nbsp;</td>
        <td class="container" style="font-family: Helvetica, sans-serif; font-size: 16px; vertical-align: top; max-width: 600px; padding: 24px 0px 24px 0px; width: 600px; margin: 0 auto;" width="600" valign="top">
          <div class="content" style="box-sizing: border-box; display: block; margin: 0 auto; max-width: 600px; padding: 0;">

            <!-- START CENTERED WHITE CONTAINER -->
            <table role="presentation" border="0" cellpadding="0" cellspacing="0" class="main" style="border-collapse: separate; mso-table-lspace: 0pt; mso-table-rspace: 0pt; background: #ffffff; border: 1px solid #eaebed; border-radius: 16px; width: 100%;" width="100%">

              <!-- START MAIN CONTENT AREA -->
              <tr>
                <td class="wrapper" style="font-family: Helvetica, sans-serif; font-size: 16px; vertical-align: top; box-sizing: border-box; padding: 24px;" valign="top">
                  <img src="` + netmakerLogoTeal + `" alt="Netmaker Logo" width="200" height="100" border="0" style="border:0; outline:none; text-decoration:none; display:block; margin-left: auto;">
                  ` + bodyContent + `
                </td>
              </tr>

              <!-- END MAIN CONTENT AREA -->
              </table>

<!-- END CENTERED WHITE CONTAINER --></div>
        </td>
        <td style="font-family: Helvetica, sans-serif; font-size: 16px; vertical-align: top;" valign="top">&nbsp;</td>
      </tr>
    </table>
  </body>
</html>`
}

func styledParagraph(text string) string {
	return `<p style="font-family: Helvetica, sans-serif; font-size: 16px; font-weight: normal; margin: 0; margin-bottom: 16px;">` + text + `</p>`
}
