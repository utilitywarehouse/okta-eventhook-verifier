# okta-eventhook-verifier

A okta event hook handler which only performs initial event hook verification
required by okta.
This service only handles one-off verification request.

### [Verify your endpoint](https://developer.okta.com/docs/concepts/event-hooks/#verify-your-endpoint)
>After registering the event hook, you need to trigger a one-time verification process by clicking the Verify button in the Admin Console. When you trigger a verification, Okta calls out to your external service, making the one-time verification request to it. You need to have implemented functionality in your service to handle the expected request and response. The purpose of this step is to prove that you control the endpoint.

## config

* --listen-address (LISTEN_ADDRESS) - (default: :9000) 
  The address the web server binds to.

* --event-hook-paths (EVENT_HOOK_PATHS) - (default: "/") 
  comma separated string of event hook paths. 
  service will handle verification requests on these paths.

* --exit-when-done (EXIT_WHEN_DONE) - (default: true) 
  if true service will exit once *all* event hooks (paths) are verified

* --time-out-hours (TIME_OUT_HOURS) - (default: 0) 
  if value is greater then 0 then service will exit in given hours even if event hooks are not verified.


### examples

* register 1 event hook and only exit once event hook verified
  ```sh
  ❯ ./okta-eventhook-verifier \
      --event-hook-paths="/eventhook/test1" \
  ```

* register 2 event hooks and exit when both are verified or after 3 hours
  ```sh
  ❯ ./okta-eventhook-verifier \
      --event-hook-paths="/eventhook/test1,/eventhook/test2" \
      --exit-when-done=true \
      --time-out-hours=3 
  ```
* register 1 event hook and never exit
  ```sh
  ❯ ./okta-eventhook-verifier \
      --event-hook-paths="/" \
      --exit-when-done=false \
      --time-out-hours=0
  ```
