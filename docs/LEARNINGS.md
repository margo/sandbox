# Important larnings/considerations/challenges that dev-team observed are documented here:

- Significance of [dev-repo]
- - This [repo](https://github.com/margo/dev-repo/tree/main/poc/device/agent) holds the PoC implementations for device-agent.
- - [OpenAPI spec](https://github.com/margo/dev-repo/tree/main/standard) created from Margo's official docs.
- - [OpenAPI spec](https://github.com/margo/dev-repo/tree/main/non-standard) created to support the operations for PoC which are not defined in the official spec.
- - [Some shared-lib utilities](https://github.com/margo/dev-repo/tree/main/shared-lib) that can be helpful to anybody.

- Significance of [https://github.com/margo/symphony]
- - This repo is a clone of the official [Eclipse Symphony repository](https://github.com/eclipse-symphony/) branch: main, just that Margo PoC is added on top of it.
- - The PoC implementation of be found [here](https://github.com/margo/symphony/tree/margo-dev-sprint-6)

- Any relation between [dev-repo] and [symphony repo] ?
Yup! The symphony repo implements the official and supportive unofficial spec by referring the code available in the dev-repo.
This was done on purpose, so that anybody else who's writing or making their WFM compliant can reuse the code, if needed, obviously might not be production grade.

- What else is the use of [dev-repo] ?
If you are a WFM or device vendor, a lot of code from this repo can be helpful for you, obviously might not be production grade.

# The things you need to understand before diving into this PoC or developing your own : )

## Device Onboarding
The device onboarding journey is still immature in the official docs hence it is included in this PoC in somewhat similar flavour. We have borrowed some ideas and implemented them where the device is onboarded using POST /onboarding RESTful endpoint but since the payload is not defined in the official spec (https://specification.margo.org/margo-api-reference/workload-api/onboarding-api/device-onboarding/) hence we have passed a unqiue device signature in the request for now so that the device can pose its unique identity(as if a true OEM device with unique signature would do) to the wfm, the wfm reads this signature and accepts it(without any filteration logic, will refine as the onboarding journey becomes clearer down the line) and then registers this device within its database.

In this way, the devices can auto-onboard themselves given that they have unique signatures and not conflicting with any other device's signature, in latter case the wfm will raise a Conflict HTTP status code. As already mentioned we have not added any whitelisting/blacklisting/filteration on these signatures that can allow/deny certain devices. All signatures are accepted by the corresponding wfm implementation.

Once onboarding is finished, the wfm returns back creds (clientid, secret, tokenurl) that can be used to obtain OAuth token, that will be used in all auth protected wfm endpoints.

NOTE: If you have any proposals, or objections, please route them through the official stakeholders of Margo. These links could be of help: 
- https://specification.margo.org/how-to-contribute/contribution-tutorial/ 
- https://specification.margo.org/how-to-contribute/contribution-navigation/

## App Package Handling
The application package is uploaded on a Git repo as of now. [This is also under discussion, hence please get in touch with stakeholders or refer the official docs].
This Git repo should have a defined structure, where a margo.yaml file should be present with valid [Margo Application Description Manifest](https://specification.margo.org/app-interoperability/application-package-definition/) accompanied with resources(like icons, license etc...). So, now this Git repo is ready and hosting your application package.

After you have created a Git repo, now is the time to seed this Git repo details in the WFM, so that it can fetch the application package and store it in its datastore for further use.

Further, to seed this Git repo details we have implemented special RESTful APIs on WFM, you can see its [OpenAPI spec here](https://github.com/margo/dev-repo/tree/dev-sprint-6/non-standard/spec). Anyways, once seeded, the WFM will fetch the content from the repo, and will store the details(it's completely upto WFM whether to store them as is, or convert them into native objects, the current PoC for Eclipse Symphony does both) in its datastore.

Once uploaded the WFM returns back a unique identifier for this app package. This id will be used to address this app-package within WFM, in any further LCM operations.

Note: This WFM API is unofficial and is not part of the Margo spec. It is implemented to give you an idea on how WFMs can inject a package. And, please don't think of any Gitops pattern between Git Repo and WFM as of now, as the official story needs approval/rejection. Check note given in [this url](https://specification.margo.org/app-interoperability/workload-orch-to-app-reg-interaction/?h=gitops) .


## Deployment & Desired State
Now, we have a unique identifier for our application package. Next comes, the part where we'll need to deploy it. What API should be present on the WFM is not defined in the Margo spec, hence we defined an unofficial one and its spec is [over here](https://github.com/margo/dev-repo/tree/dev-sprint-6/non-standard/spec) (we wanted to use the existing APIs of Symphony, but couldn't use them very well for this PoC, hence to unblock ourselves we created new endpoints, the main motive is to give a good idea of how the deployment will happen).

Anyways, the Margo still defines a spec but the one between the WFM and Device Agent. We call it as "Desired State API". We'll come back to that part in some time.

So, now we'll be sending a request to the WFM to deploy the application package, by mentioning the App Package Id, and the Device Id in it, so that WFM gets to know which app package to deployed on which device. Again this is just an idea, your WFM can do it some different way.

Once the request is received on the WFM, it stores it its datastore.

But it didn't get deployed yet! What??!

Okay, so here comes the story from the device agent side. This is defined in the [official Margo specs](https://specification.margo.org/margo-api-reference/workload-api/desired-state-api/desired-state/). The gist is that the device reach out to WFM, not vice-versa, to report the current states of the deployments that it is managing, and the WFM will compare them and will return back the desired states of these deployments on the device. Now, the device tries to acheive the same desired states by doing operations. The desired could be to run a new deployment, delete an existing deployment, upgrade an existing deployment. Please refer the official docs to see what desired states could be there.

Finally, the device-agent deploys the app for us. : )

## Deployment Status
The running deployments can transition themselves into different states. For example, a running app might crash, or it might get stuck in some pending state etc.
Hence, the device-agent should send this info to the WFM, so that WFM also knows what's happening on the device. So, the offical Margo spec provides an WFM API that can be hit by the device-agents to report the deployment status. You can find the official notes (here)[https://specification.margo.org/margo-api-reference/workload-api/device-api/deployment-status/], and the PoC OpenAPI spec (here)[https://github.com/margo/dev-repo/tree/dev-sprint-6/standard/spec] .

## Device Capabilities
Great! Now our app is running fine. But there are couple of other good things that we can have. One of them is that if the device sends what capabilities it has. This will help the WFM to make better decisions while scheduling a deployment etc... Completely upto them!

Anyways, the [official Margo spec](https://specification.margo.org/margo-api-reference/workload-api/onboarding-api/device-onboarding/) defines a WFM API that can be hit by the device-agents to report what capabilities they have. The current device-agent in the PoC, reads the capabilities from a file. This is upto the device owner what they want to put into the capabilities file, and it'll be submitted to the WFM, as long as it follows the spec defined [here](https://specification.margo.org/margo-api-reference/workload-api/device-api/device-capabilities/).

Once the device-agent submits the capabilities, the WFM stores them in its datastore, and as of now the PoC WFM doesn't use this info to make any important decisions.

## What is not yet part of the PoC?
- [Certificate API](https://specification.margo.org/margo-api-reference/workload-api/onboarding-api/rootca-download/) as we need some clarity on this.

## Where to go from here?
Please consider this PoC to be having some flaws in it, for example, the validations might not work, some error logs still might be bogus, or your application might not get deployed as expected, in such a case would appreciate any contributions.

Thanks a lot to the TWG folks and you for reading this. Good day!  : )
Dev-Team