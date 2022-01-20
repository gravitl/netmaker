# How to contribute to Netmaker

Hey! We're glad you're here. We need your help making Netmaker as great as it can be.

If you haven't already, come chat with us in [Discord](https://discord.gg/zRb9Vfhk8A). We can help you find the right thing to work on.

Before you start contributing, take a moment to check here if it makes sense.

#### **Did you find a bug?**

* Search on on GitHub under [Issues](https://github.com/gravitl/netmaker/issues) to make sure the bug was not already discovered.

* If you don't find an open issue that addresses the problem, you can [open a new one](https://github.com/gravitl/netmaker/issues/new). 

* If you're creating a new issue, include a **title and clear description**, as much relevant information as possible **including logs**, and an explanation/output demonstrating expected behavior vs. actual behavior. Make sure to specify the **version of netmaker/netclient.** If it's a server issue, describe the environment where the server is running. If it's a client issue, give us the operating system and any relevant environment factors (CGNAT, 4g router, etc).

* Respond to team queries in a timely manner, since stale issues will be closed.

#### **Did you write a patch that fixes a bug?**

* Open a new GitHub pull request with the patch **against the 'develop' branch**.

* The PR should clearly describe the problem and solution. Include an issue number if possible.

* Make sure to add comments for any new functions

#### **Did you fix whitespace, format code, or make a purely cosmetic patch?**

Cosmetic changes that do not add substantial useability, stability, functionality, or testability to the code base will not be accepted. The calculation is simple. If it will take more time to merge and test than it took you to make and submit the code, it is likely not worthwhile (execptions exist of course for critical errors with easy fixes).

#### **Do you want to add a new feature to Netmaker?**

* Once again, join the [Discord](https://discord.gg/zRb9Vfhk8A)! Bring it up there and we can discuss. Even if you do not know what you want to build, but you want to build something, we can help you choose something from the roadmap.

#### **Do you want to contribute to Netmaker documentation?**

* Make sure your documentation compiles correctly

* You will need [sphinx](https://www.sphinx-doc.org/en/master/usage/installation.html) and the [material theme](https://github.com/bashtage/sphinx-material/) to run the documentation locally.

* Once the above plugins are installed, you can navigate to the **docs** directory and run **make html**

* View the compiled files (start with index.html under _build) in your browser and make sure your changes look correct before submitting.


## Submitting Changes

* Please label your branch using our convention: **purpose_version_thing-you-did**. Purpose is either feature, bugfix, or hotfix.

* Examples: feature_v0.9.5_widget, bugfix_v0.8.2_ipv6-changes

* Please open a [Pull Request](https://github.com/gravitl/netmaker/compare/develop...master?expand=1) against the develop branch with your branch which clearly describes everything you've done and references any related GitHub issues. 

* You will need to sign the CLA in order for us to accept your changes (a bot should appear asking you to sign)

* Please respond to any feedback in a timely manner. Stale PR's will be closed periodically.

## Coding conventions

Take a look around the code to get a feel for how we're doing things.

* Use private functions where possible
* Use the custom loggers for log messages
* Comment any new public functions




## Thanks for taking the time to read this! You're awesome, and we look forward to working with you!
  
-The Netmaker Team
