package email

import "strings"

// mail related images hosted on github
var (
	nLogoTeal        = "https://raw.githubusercontent.com/gravitl/netmaker/netmaker_logos/img/logos/N_Teal.png"
	netmakerLogoTeal = "https://raw.githubusercontent.com/gravitl/netmaker/netmaker_logos/img/logos/netmaker-logo-2.png"
	netmakerMeshLogo = "https://raw.githubusercontent.com/gravitl/netmaker/netmaker_logos/img/logos/netmaker-mesh.png"
	linkedinIcon     = "https://raw.githubusercontent.com/gravitl/netmaker/netmaker_logos/img/logos/linkedin2x.png"
	discordIcon      = "https://raw.githubusercontent.com/gravitl/netmaker/netmaker_logos/img/logos/discord-logo-png-7617.png"
	githubIcon       = "https://raw.githubusercontent.com/gravitl/netmaker/netmaker_logos/img/logos/Octocat.png"
	mailIcon         = "https://raw.githubusercontent.com/gravitl/netmaker/netmaker_logos/img/logos/icons8-mail-24.png"
	addressIcon      = "https://raw.githubusercontent.com/gravitl/netmaker/netmaker_logos/img/logos/icons8-address-16.png"
	linkIcon         = "https://raw.githubusercontent.com/gravitl/netmaker/netmaker_logos/img/logos/icons8-hyperlink-64.png"
)

type EmailBodyBuilder interface {
	WithHeadline(text string) EmailBodyBuilder
	WithParagraph(text string) EmailBodyBuilder
	WithSignature() EmailBodyBuilder
	Build() string
}

type EmailBodyBuilderWithH1HeadlineAndImage struct {
	headline     string
	paragraphs   []string
	hasSignature bool
}

func (b *EmailBodyBuilderWithH1HeadlineAndImage) WithHeadline(text string) EmailBodyBuilder {
	b.headline = text
	return b
}

func (b *EmailBodyBuilderWithH1HeadlineAndImage) WithParagraph(text string) EmailBodyBuilder {
	b.paragraphs = append(b.paragraphs, text)
	return b
}

func (b *EmailBodyBuilderWithH1HeadlineAndImage) WithSignature() EmailBodyBuilder {
	b.hasSignature = true
	return b
}

func (b *EmailBodyBuilderWithH1HeadlineAndImage) Build() string {
	// map paragraphs to styled paragraphs
	styledParagraphsSlice := make([]string, len(b.paragraphs))
	for i, paragraph := range b.paragraphs {
		styledParagraphsSlice[i] = styledParagraph(paragraph)
	}
	// join styled paragraphs
	styledParagraphsString := strings.Join(styledParagraphsSlice, "")

	signature := ""
	if b.hasSignature {
		signature = styledSignature()
	}

	return `
		<!DOCTYPE html>
		<html xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office" lang="en">
		<head>
		    <title></title>
		    <meta http-equiv="Content-Type" content="text/html; charset=utf-8">
		    <meta name="viewport" content="width=device-width,initial-scale=1">
		    <!--[if mso]>
		    <xml>
		        <o:OfficeDocumentSettings>
		            <o:PixelsPerInch>96</o:PixelsPerInch>
		            <o:AllowPNG/>
		        </o:OfficeDocumentSettings>
		    </xml>
		    <![endif]-->
		    <style>
		        *{box-sizing:border-box}body{margin:0;padding:0}a[x-apple-data-detectors]{color:inherit!important;text-decoration:inherit!important}#MessageViewBody a{color:inherit;text-decoration:none}p{line-height:inherit}.desktop_hide,.desktop_hide table{mso-hide:all;display:none;max-height:0;overflow:hidden}@media (max-width:720px){.desktop_hide table.icons-inner{display:inline-block!important}.icons-inner{text-align:center}.icons-inner td{margin:0 auto}.image_block img.big,.row-content{width:100%!important}.mobile_hide{display:none}.stack .column{width:100%;display:block}.mobile_hide{min-height:0;max-height:0;max-width:0;overflow:hidden;font-size:0}.desktop_hide,.desktop_hide table{display:table!important;max-height:none!important}} .x-button{background:#5E5DF0;border-radius:999px;box-shadow:#5E5DF0 0 10px 20px -10px;box-sizing:border-box;color:#FFFFFF !important;cursor:pointer;font-family:Inter,Helvetica,"Apple Color Emoji","Segoe UI Emoji",NotoColorEmoji,"Noto Color Emoji","Segoe UI Symbol","Android Emoji",EmojiSymbols,-apple-system,system-ui,"Segoe UI",Roboto,"Helvetica Neue","Noto Sans",sans-serif;font-size:16px;font-weight:700;line-height:24px;opacity:1;outline:0 solid transparent;padding:8px 18px;user-select:none;-webkit-user-select:none;touch-action:manipulation;width:fit-content;word-break:break-word;border:0;margin:20px 20px 20px 0px;text-decoration:none;}
		    </style>
		</head>
		<body style="background-color:transparent;margin:0;padding:0;-webkit-text-size-adjust:none;text-size-adjust:none">
		<table class="nl-container" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0;background-color:transparent">
		    <tbody>
		    <tr>
		        <td>
		            <table class="row row-1" align="center" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                <tbody>
		                <tr>
		                    <td>
		                        <table class="row-content" align="center" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0;color:#000;width:700px" width="700">
		                            <tbody>
		                            <tr>
		                                <td class="column column-1" width="50%" style="mso-table-lspace:0;mso-table-rspace:0;font-weight:400;text-align:left;vertical-align:top;border-top:0;border-right:0;border-bottom:0;border-left:0">
		                                    <table class="image_block block-2" width="100%" border="0" cellpadding="0" cellspacing="0"
		                                           role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                                        <tr>
		                                            <td class="pad" style="padding-left:15px;padding-right:15px;width:100%;padding-top:5px">
		                                                <div class="alignment" align="left" style="line-height:10px"><a href="https://www.netmaker.io/" target="_blank" style="outline:none" tabindex="-1"><img class="big" src="` + netmakerLogoTeal + `"
		                                                                                                                                                                                                        style="display:block;height:auto;border:0;width:333px;max-width:100%" width="333" alt="Netmaker" title="Netmaker"></a></div>
		                                            </td>
		                                        </tr>
		                                    </table>
		                                    <table class="divider_block block-3" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                                        <tr>
		                                            <td class="pad" style="padding-bottom:10px;padding-left:5px;padding-right:5px;padding-top:10px">
		                                                <div class="alignment" align="center">
		                                                    <table border="0" cellpadding="0" cellspacing="0"
		                                                           role="presentation" width="100%" style="mso-table-lspace:0;mso-table-rspace:0">
		                                                        <tr>
		                                                            <td class="divider_inner" style="font-size:1px;line-height:1px;border-top:0 solid #bbb"><span>&#8202;</span></td>
		                                                        </tr>
		                                                    </table>
		                                                </div>
		                                            </td>
		                                        </tr>
		                                    </table>
		                                </td>
		                                <td class="column column-2" width="50%" style="mso-table-lspace:0;mso-table-rspace:0;font-weight:400;text-align:left;vertical-align:top;border-top:0;border-right:0;border-bottom:0;border-left:0">
		                                    <table class="empty_block block-2" width="100%" border="0"
		                                           cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                                        <tr>
		                                            <td class="pad" style="padding-right:0;padding-bottom:5px;padding-left:0;padding-top:5px">
		                                                <div></div>
		                                            </td>
		                                        </tr>
		                                    </table>
		                                </td>
		                            </tr>
		                            </tbody>
		                        </table>
		                    </td>
		                </tr>
		                </tbody>
		            </table>
		            <table class="row row-2" align="center" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                <tbody>
		                <tr>
		                    <td>
		                        <table class="row-content stack" align="center"
		                               border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0;color:#000;width:700px" width="700">
		                            <tbody>
		                            <tr>
		                                <td class="column column-1" width="100%" style="mso-table-lspace:0;mso-table-rspace:0;font-weight:400;text-align:left;padding-left:10px;padding-right:10px;vertical-align:top;padding-top:10px;padding-bottom:10px;border-top:0;border-right:0;border-bottom:0;border-left:0">
		                                    <table class="divider_block block-1" width="100%" border="0"
		                                           cellpadding="10" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                                        <tr>
		                                            <td class="pad">
		                                                <div class="alignment" align="center">
		                                                    <table border="0" cellpadding="0" cellspacing="0" role="presentation" width="100%" style="mso-table-lspace:0;mso-table-rspace:0">
		                                                        <tr>
		                                                            <td class="divider_inner" style="font-size:1px;line-height:1px;border-top:0 solid #bbb"><span>&#8202;</span></td>
		                                                        </tr>
		                                                    </table>
		                                                </div>
		                                            </td>
		                                        </tr>
		                                    </table>
		                                </td>
		                            </tr>
		                            </tbody>
		                        </table>
		                    </td>
		                </tr>
		                </tbody>
		            </table>
		            <table
		                    class="row row-3" align="center" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                <tbody>
		                <tr>
		                    <td>
		                        <table class="row-content stack" align="center" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0;color:#000;width:700px" width="700">
		                            <tbody>
		                            <tr>
		                                <td class="column column-1" width="50%"
		                                    style="mso-table-lspace:0;mso-table-rspace:0;font-weight:400;text-align:left;vertical-align:top;border-top:0;border-right:0;border-bottom:0;border-left:0">
		                                    <table class="divider_block block-2" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                                        <tr>
		                                            <td class="pad" style="padding-bottom:20px;padding-left:20px;padding-right:20px;padding-top:25px">
		                                                <div class="alignment" align="center">
		                                                    <table border="0" cellpadding="0"
		                                                           cellspacing="0" role="presentation" width="100%" style="mso-table-lspace:0;mso-table-rspace:0">
		                                                        <tr>
		                                                            <td class="divider_inner" style="font-size:1px;line-height:1px;border-top:0 solid #bbb"><span>&#8202;</span></td>
		                                                        </tr>
		                                                    </table>
		                                                </div>
		                                            </td>
		                                        </tr>
		                                    </table>
		                                    <table class="heading_block block-3" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                                        <tr>
		                                            <td class="pad"
		                                                style="padding-bottom:15px;padding-left:10px;padding-right:10px;padding-top:10px;text-align:center;width:100%">
		                                                <h1 style="margin:0;color:#2b2d2d;direction:ltr;font-family:'Helvetica Neue',Helvetica,Arial,sans-serif;font-size:28px;font-weight:400;letter-spacing:normal;line-height:120%;text-align:left;margin-top:0;margin-bottom:0"><strong>` + b.headline + `</strong></h1>
		                                            </td>
		                                        </tr>
		                                    </table>
		                                </td>
		                                <td class="column column-2" width="50%"
		                                    style="mso-table-lspace:0;mso-table-rspace:0;font-weight:400;text-align:left;vertical-align:top;border-top:0;border-right:0;border-bottom:0;border-left:0">
		                                    <table class="image_block block-2" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                                        <tr>
		                                            <td class="pad" style="width:100%;padding-right:0;padding-left:0;padding-top:5px;padding-bottom:5px">
		                                                <div class="alignment" align="center" style="line-height:10px"><img
		                                                        src="` + netmakerMeshLogo + `" style="display:block;height:auto;border:0;width:350px;max-width:100%" width="350" alt="Netmaker Mesh"></div>
		                                            </td>
		                                        </tr>
		                                    </table>
		                                </td>
		                            </tr>
		                            </tbody>
		                        </table>
		                    </td>
		                </tr>
		                </tbody>
		            </table>
		            <table class="row row-4" align="center" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation"
		                   style="mso-table-lspace:0;mso-table-rspace:0">
		                <tbody>
		                <tr>
		                    <td>
		                        <table class="row-content stack" align="center" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0;color:#000;width:700px" width="700">
		                            <tbody>
		                            <tr>
		                                <td class="column column-1" width="100%" style="mso-table-lspace:0;mso-table-rspace:0;font-weight:400;text-align:left;vertical-align:top;padding-top:5px;padding-bottom:5px;border-top:0;border-right:0;border-bottom:0;border-left:0">
		                                    <table class="divider_block block-1" width="100%" border="0" cellpadding="10" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                                        <tr>
		                                            <td class="pad">
		                                                <div class="alignment" align="center">
		                                                    <table border="0" cellpadding="0" cellspacing="0" role="presentation" width="100%" style="mso-table-lspace:0;mso-table-rspace:0">
		                                                        <tr>
		                                                            <td class="divider_inner" style="font-size:1px;line-height:1px;border-top:0 solid #bbb"><span>&#8202;</span></td>
		                                                        </tr>
		                                                    </table>
		                                                </div>
		                                            </td>
		                                        </tr>
		                                    </table>
		                                </td>
		                            </tr>
		                            </tbody>
		                        </table>
		                    </td>
		                </tr>
		                </tbody>
		            </table>
		            <table class="row row-5" align="center" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                <tbody>
		                <tr>
		                    <td>
		                        <table class="row-content stack" align="center" border="0" cellpadding="0" cellspacing="0" role="presentation"
		                               style="mso-table-lspace:0;mso-table-rspace:0;background-color:#0098a5;color:#000;border-top:2px solid transparent;border-right:2px solid transparent;border-left:2px solid transparent;border-bottom:2px solid transparent;border-radius:0;width:700px" width="700">
		                            <tbody>
		                            <tr>
		                                <td class="column column-1" width="100%"
		                                    style="mso-table-lspace:0;mso-table-rspace:0;font-weight:400;text-align:left;border-bottom:0 solid #000;border-left:0 solid #000;border-right:0 solid #000;border-top:0 solid #000;vertical-align:top;padding-top:25px;padding-bottom:25px">
		                                    <table class="text_block block-3" width="100%" border="0"
		                                           cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0;word-break:break-word">
		                                        <tr>
		                                            <td class="pad" style="padding-bottom:10px;padding-left:50px;padding-right:50px;padding-top:10px">
		                                                <div style="font-family:Verdana,sans-serif">
		                                                    <div class="txtTinyMce-wrapper" style="font-size:12px;mso-line-height-alt:18px;color:#393d47;line-height:1.5;font-family:Verdana,Geneva,sans-serif">
		
		                                                        <p style="margin:0;font-size:12px;mso-line-height-alt:18px">&nbsp;</p>
		                                                        ` + styledParagraphsString + `
		                                                        <p style="margin:0;mso-line-height-alt:18px">&nbsp;</p>
		                                                    </div>
		                                                </div>
		                                            </td>
		                                        </tr>
		                                    </table>
		                                </td>
		                            </tr>
		                            </tbody>
		                        </table>
		                    </td>
		                </tr>
		                </tbody>
		            </table>
		            <table class="row row-6" align="center" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                <tbody>
		                <tr>
		                    <td>
		                        <table
		                                class="row-content stack" align="center" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0;color:#000;width:700px" width="700">
		                            <tbody>
		                            <tr>
		                                <td class="column column-1" width="100%" style="mso-table-lspace:0;mso-table-rspace:0;font-weight:400;text-align:left;vertical-align:top;padding-top:5px;padding-bottom:5px;border-top:0;border-right:0;border-bottom:0;border-left:0">
		                                    <table class="divider_block block-1" width="100%" border="0"
		                                           cellpadding="10" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                                        <tr>
		                                            <td class="pad">
		                                                <div class="alignment" align="center">
		                                                    <table border="0" cellpadding="0" cellspacing="0" role="presentation" width="100%" style="mso-table-lspace:0;mso-table-rspace:0">
		                                                        <tr>
		                                                            <td class="divider_inner" style="font-size:1px;line-height:1px;border-top:0 solid #bbb"><span>&#8202;</span></td>
		                                                        </tr>
		                                                    </table>
		                                                </div>
		                                            </td>
		                                        </tr>
		                                    </table>
		                                </td>
		                            </tr>
		                            </tbody>
		                        </table>
		                    </td>
		                </tr>
		                </tbody>
		            </table>
		            <table
		                    class="row row-7" align="center" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0;background-color:#f7fafe">
		                <tbody>
		                <tr>
		                    <td>
		                        <table class="row-content stack" align="center" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0;color:#000;width:700px" width="700">
		                            <tbody>
		                            <tr>
		                                <td class="column column-1" width="100%"
		                                    style="mso-table-lspace:0;mso-table-rspace:0;font-weight:400;text-align:left;vertical-align:top;padding-top:25px;padding-bottom:5px;border-top:0;border-right:0;border-bottom:0;border-left:0">
		                                    <table class="divider_block block-1" width="100%" border="0" cellpadding="10" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                                        <tr>
		                                            <td class="pad">
		                                                <div class="alignment" align="center">
		                                                    <table border="0" cellpadding="0" cellspacing="0" role="presentation" width="100%"
		                                                           style="mso-table-lspace:0;mso-table-rspace:0">
		                                                        <tr>
		                                                            <td class="divider_inner" style="font-size:1px;line-height:1px;border-top:0 solid #bbb"><span>&#8202;</span></td>
		                                                        </tr>
		                                                    </table>
		                                                </div>
		                                            </td>
		                                        </tr>
		                                    </table>
		                                </td>
		                            </tr>
		                            </tbody>
		                        </table>
		                    </td>
		                </tr>
		                </tbody>
		            </table>
		            <table class="row row-8" align="center" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0;background-color:#090660">
		                <tbody>
		                <tr>
		                    <td>
		                        <table class="row-content stack"
		                               align="center" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0;color:#000;width:700px" width="700">
		                            <tbody>
		                            <tr>
		                                <td class="column column-1" width="100%" style="mso-table-lspace:0;mso-table-rspace:0;font-weight:400;text-align:left;vertical-align:top;padding-top:5px;padding-bottom:5px;border-top:0;border-right:0;border-bottom:0;border-left:0">
		                                    <table class="text_block block-1" width="100%" border="0" cellpadding="0" cellspacing="0"
		                                           role="presentation" style="mso-table-lspace:0;mso-table-rspace:0;word-break:break-word">
		                                        <tr>
		                                            <td class="pad" style="padding-bottom:10px;padding-left:50px;padding-right:50px;padding-top:10px">
		                                                <div style="font-family:sans-serif">
		                                                    <div class="txtTinyMce-wrapper" style="font-size:12px;mso-line-height-alt:18px;color:#6f7077;line-height:1.5;font-family:Arial,Helvetica Neue,Helvetica,sans-serif">
		                                                        <p style="margin:0;font-size:12px;mso-line-height-alt:33px">
		                                                            <span style="color:#ffffff;font-size:22px;">&nbsp; &nbsp; &nbsp; &nbsp; &nbsp; &nbsp; &nbsp; &nbsp; &nbsp; &nbsp; &nbsp; &nbsp; &nbsp; &nbsp; &nbsp; &nbsp;Get In Touch With Us</span>
		                                                        </p>
		                                                    </div>
		                                                </div>
		                                            </td>
		                                        </tr>
		                                    </table>
		                                    <table class="social_block block-2" width="100%" border="0" cellpadding="10" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                                        <tr>
		                                            <td class="pad">
		                                                <div class="alignment" style="text-align:center">
		                                                    <table class="social-table"
		                                                           width="114.49624060150376px" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0;display:inline-block">
		                                                        <tr>
		                                                            <td style="padding:0 2px 0 2px"><a href="https://www.linkedin.com/company/netmaker-inc/" target="_blank"><img src="` + linkedinIcon + `" width="32" height="32" alt="Linkedin" title="linkedin" style="display:block;height:auto;border:0"></a></td>
		                                                            <td
		                                                                    style="padding:0 2px 0 2px"><a href="https://discord.gg/zRb9Vfhk8A" target="_blank"><img src="` + discordIcon + `" width="32" height="32" alt="Discord" title="Discord" style="display:block;height:auto;border:0"></a></td>
		                                                            <td style="padding:0 2px 0 2px"><a href="https://github.com/gravitl/netmaker" target="_blank"><img
		                                                                    src="` + githubIcon + `" width="38.49624060150376" height="32" alt="Github" title="Github" style="display:block;height:auto;border:0"></a></td>
		                                                        </tr>
		                                                    </table>
		                                                </div>
		                                            </td>
		                                        </tr>
		                                    </table>
		                                </td>
		                            </tr>
		                            </tbody>
		                        </table>
		                    </td>
		                </tr>
		                </tbody>
		            </table>
		            <table class="row row-9" align="center" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                <tbody>
		                <tr>
		                    <td>
		                        <table
		                                class="row-content stack" align="center" border="0" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0;color:#000;width:700px" width="700">
		                            <tbody>
		                            <tr>
		                                <td class="column column-1" width="100%" style="mso-table-lspace:0;mso-table-rspace:0;font-weight:400;text-align:left;vertical-align:top;padding-top:5px;padding-bottom:5px;border-top:0;border-right:0;border-bottom:0;border-left:0">
		                                    <table class="icons_block block-1" width="100%" border="0"
		                                           cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                                        <tr>
		                                            <td class="pad" style="vertical-align:middle;padding-bottom:5px;padding-top:5px;text-align:center;color:#9d9d9d;font-family:inherit;font-size:15px">
		                                                <table width="100%" cellpadding="0" cellspacing="0" role="presentation" style="mso-table-lspace:0;mso-table-rspace:0">
		                                                    <tr>
		                                                        <td class="alignment" style="vertical-align:middle;text-align:center">
		                                                            <!--[if vml]>
		                                                            <table align="left" cellpadding="0" cellspacing="0" role="presentation" style="display:inline-block;padding-left:0px;padding-right:0px;mso-table-lspace: 0pt;mso-table-rspace: 0pt;">
		                                                            <![endif]--><!--[if !vml]><!-->
		                                                            <table class="icons-inner" style="mso-table-lspace:0;mso-table-rspace:0;display:inline-block;margin-right:-4px;padding-left:0;padding-right:0" cellpadding="0" cellspacing="0" role="presentation">
		                                                                <!--<![endif]-->
		                                                            </table>
		                                                        </td></tr>
		                                                </table>
		                                            </td>
		                                        </tr>
		                                    </table>
		                                </td>
		                            </tr>
		                            </tbody>
		                        </table>
		                    </td>
		                </tr>
		                </tbody>
		            </table>
		        </td>
		    </tr>
		    </tbody>
		</table>
		<!-- End -->
		</body>
		` + signature + `
		</html>`
}

func styledSignature() string {
	return `
	<footer style="display:block">
	<table cellpadding="0" cellspacing="0" class="sc-gPEVay eQYmiW" style="vertical-align: -webkit-baseline-middle; font-size: medium; font-family: Arial;">
	<tbody>
	   <tr>
		  <td>
			 <table cellpadding="0" cellspacing="0" class="sc-gPEVay eQYmiW" style="vertical-align: -webkit-baseline-middle; font-size: medium; font-family: Arial;">
				<tbody>
				   <tr>
					  <td style="vertical-align: top;">
						 <table cellpadding="0" cellspacing="0" class="sc-gPEVay eQYmiW" style="vertical-align: -webkit-baseline-middle; font-size: medium; font-family: Arial;">
							<tbody>
							   <tr>
								  <td class="sc-TOsTZ kjYrri" style="text-align: center;"><img src="` + nLogoTeal + `" role="presentation" width="130" class="sc-cHGsZl bHiaRe" style="max-width: 130px; display: block;"></td>
							   </tr>
							   <tr>
								  <td height="30"></td>
							   </tr>
							   <tr>
								  <td style="text-align: center;">
									 <table cellpadding="0" cellspacing="0" class="sc-gPEVay eQYmiW" style="vertical-align: -webkit-baseline-middle; font-size: medium; font-family: Arial; display: inline-block;">
										<tbody>
										   <tr style="text-align: center;">
											  <td><a href="https://www.linkedin.com/company/netmaker-inc/" color="#6a78d1" class="sc-hzDkRC kpsoyz" style="display: inline-block; padding: 0px; background-color: rgb(106, 120, 209);"><img src="` + linkedinIcon + `" alt="Linkedin" color="#6a78d1" height="24" class="sc-bRBYWo ccSRck" style="background-color: rgb(106, 120, 209); max-width: 135px; display: block;"></a></td>
											  <td width="5">
												 <div></div>
											  </td>
										 
                                    <td><a href="https://discord.gg/zRb9Vfhk8A" class="sc-hzDkRC kpsoyz" style="display: inline-block; padding: 0px;"><img src="` + discordIcon + `" alt="Discord" height="24" class="sc-bRBYWo ccSRck" style="max-width: 135px; display: block;"></a></td>
                                    <td width="5">
                                    <div></div>
                                    </td>
                              
                                    <td><a href="https://github.com/gravitl/netmaker" class="sc-hzDkRC kpsoyz" style="display: inline-block; padding: 0px;"><img src="` + githubIcon + `" alt="Github" height="24" class="sc-bRBYWo ccSRck" style="max-width: 135px; display: block;"></a></td>
                                    <td width="5">
                                    <div></div>
                                    </td>
                                 </tr>
										</tbody>
									 </table>
								  </td>
							   </tr>
							</tbody>
						 </table>
					  </td>
					  <td width="46">
						 <div></div>
					  </td>
					  <td style="padding: 0px; vertical-align: middle;">
						 <h3 color="#000000" class="sc-fBuWsC eeihxG" style="margin: 0px; font-size: 18px; color: rgb(0, 0, 0);"><span>Alex</span><span>&nbsp;</span><span>Feiszli</span></h3>
						 <p color="#000000" font-size="medium" class="sc-fMiknA bxZCMx" style="margin: 0px; color: rgb(0, 0, 0); font-size: 14px; line-height: 22px;"><span>Co-Founder &amp; CEO</span></p>
						 <p color="#000000" font-size="medium" class="sc-dVhcbM fghLuF" style="margin: 0px; font-weight: 500; color: rgb(0, 0, 0); font-size: 14px; line-height: 22px;"><span>Netmaker</span></p>
						 <table cellpadding="0" cellspacing="0" class="sc-gPEVay eQYmiW" style="vertical-align: -webkit-baseline-middle; font-size: medium; font-family: Arial; width: 100%;">
							<tbody>
							   <tr>
								  <td height="30"></td>
							   </tr>
							   <tr>
								  <td color="#545af2" direction="horizontal" height="1" class="sc-jhAzac hmXDXQ" style="width: 100%; border-bottom: 1px solid rgb(84, 90, 242); border-left: none; display: block;"></td>
							   </tr>
							   <tr>
								  <td height="30"></td>
							   </tr>
							</tbody>
						 </table>
						 <table cellpadding="0" cellspacing="0" class="sc-gPEVay eQYmiW" style="vertical-align: -webkit-baseline-middle; font-size: medium; font-family: Arial;">
							<tbody>
							   <tr height="25" style="vertical-align: middle;">
								  <td width="30" style="vertical-align: middle;">
									 <table cellpadding="0" cellspacing="0" class="sc-gPEVay eQYmiW" style="vertical-align: -webkit-baseline-middle; font-size: medium; font-family: Arial;">
										<tbody>
										   <tr>
											  <td style="vertical-align: bottom;"><span width="11" class="sc-jlyJG bbyJzT" style="display: block"><img src="` + mailIcon + `" width="13" class="sc-iRbamj blSEcj" style="display: block;"></span></td>
										   </tr>
										</tbody>
									 </table>
								  </td>
								  <td style="padding: 0px;"><a href="mailto:alex@netmaker.io" color="#000000" class="sc-gipzik iyhjGb" style="text-decoration: none; color: rgb(0, 0, 0); font-size: 12px;"><span>alex@netmaker.io</span></a></td>
							   </tr>
							   <tr height="25" style="vertical-align: middle;">
								  <td width="30" style="vertical-align: middle;">
									 <table cellpadding="0" cellspacing="0" class="sc-gPEVay eQYmiW" style="vertical-align: -webkit-baseline-middle; font-size: medium; font-family: Arial;">
										<tbody>
										   <tr>
											  <td style="vertical-align: bottom;"><span width="11" class="sc-jlyJG bbyJzT" style="display: block;"><img src="` + linkIcon + `" color="#545af2" width="13" class="sc-iRbamj blSEcj" style="display: block;"></span></td>
										   </tr>
										</tbody>
									 </table>
								  </td>
								  <td style="padding: 0px;"><a href="https://www.netmaker.io/" color="#000000" class="sc-gipzik iyhjGb" style="text-decoration: none; color: rgb(0, 0, 0); font-size: 12px;"><span>https://www.netmaker.io/</span></a></td>
							   </tr>
							   <tr height="25" style="vertical-align: middle;">
								  <td width="30" style="vertical-align: middle;">
									 <table cellpadding="0" cellspacing="0" class="sc-gPEVay eQYmiW" style="vertical-align: -webkit-baseline-middle; font-size: medium; font-family: Arial;">
										<tbody>
										   <tr>
											  <td style="vertical-align: bottom;"><span width="11" class="sc-jlyJG bbyJzT" style="display: block;"><img src="` + addressIcon + `"  width="13" class="sc-iRbamj blSEcj" style="display: block;"></span></td>
										   </tr>
										</tbody>
									 </table>
								  </td>
								  <td style="padding: 0px;"><span color="#000000" class="sc-csuQGl CQhxV" style="font-size: 12px; color: rgb(0, 0, 0);"><span>1465 Sand Hill Rd.Suite 2014, Candler, NC 28715</span></span></td>
							   </tr>
							</tbody>
						 </table>
						 <table cellpadding="0" cellspacing="0" class="sc-gPEVay eQYmiW" style="vertical-align: -webkit-baseline-middle; font-size: medium; font-family: Arial;">
							<tbody>
							   <tr>
								  <td height="30"></td>
							   </tr>
							</tbody>
						 </table>
					  </td>
				   </tr>
				</tbody>
			 </table>
		  </td>
	   </tr>
	</tbody>
 </table>
</footer>`
}

func styledParagraph(text string) string {
	return `<p style="margin:0;mso-line-height-alt:22.5px">
	<span style="color:#ffffff;font-size:15px;">` + text + `</span>
	</p>`
}

func GetMailSignature() string {
	return styledSignature()
}
