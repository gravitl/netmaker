===================
Integrating OAuth
====================

Introduction
==============

As of v0.8.5, Netmaker offers integration with the following OAuth providers: 

- GitHub
- Google
- Microsoft Azure AD

By integrating with an OAuth provider, your Netmaker users can log in via the provider, rather than the default simple auth.

Configuring your provider
===========================

In order to use OAuth, configure your OAuth provider (GitHub, Google, Azure AD).

You must configure your provider to use the Netmaker Dashboard URI dashboard.<netmaker.base.domain> as the origin URL.

For example: `https://dashboard.netmaker.mydomain.com`

You must configure your provider to use the Netmaker API URI redirect route with the following format: https://api.<netmaker base domain>/api/oauth2/callback.

For example: `https://api.netmaker.mydomain.com/api/oauth2/callback`

General provider instructions can be found with the following links:

Instructions for GitHub: https://oauth2-proxy.github.io/oauth2-proxy/docs/configuration/oauth_provider/#github-auth-provider
Instructions for Google: https://oauth2-proxy.github.io/oauth2-proxy/docs/configuration/oauth_provider/#google-auth-provider
Instructions for Microsoft Azure AD: https://oauth2-proxy.github.io/oauth2-proxy/docs/configuration/oauth_provider/#microsoft-azure-ad-provider 

Configuring Netmaker
======================

After you have configured your OAuth provider, take note of the CLIENT_ID and CLIENT_SECRET.

Next, Configure Netmaker with the following environment variables. If any are left blank, OAuth will fail.

.. code-block::

    AUTH_PROVIDER="<azure-ad|github|google>"
    CLIENT_ID="<client id of your oauth provider>"
    CLIENT_SECRET="<client secret of your oauth provider>"
    SERVER_API_CONN_STRING="https://<your-netmaker-api-domain>"
    FRONTEND_URL="https://<your-netmaker-dashboard-domain>"


After restarting your server, the Netmaker logs will indicate if the OAuth provider was successfully initialized.

Once successful, users can click the key symbol on the login page to sign-in with your configured OAuth provider.

.. image:: images/oauth1.png
   :width: 80%
   :alt: Login Oauth
   :align: center

Configuring User Permissions
===============================

All users logging in will have zero permissions on first sign-in. An admin must configure all user permissions.

Admins must navigate to the "Users" screen to configure permissions.

For each user, an admin must specify which networks that user has access to configure. Additionally, an Admin can elevate a user to Admin permissions.

.. image:: images/oauth2.png
   :width: 80%
   :alt: Edit User
   :align: center

.. image:: images/oauth3.png
   :width: 80%
   :alt: Edit User 2
   :align: center
