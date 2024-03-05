# Harmony

Harmony is a lighter-weight fork of [Dendrite](https://github.com/matrix-org/dendrite), a second-generation homeserver for the [Matrix](https://matrix.org/) protocol.

As with Dendrite, supported features include:

* Core room functionality (creating rooms, invites, auth rules)
* Room versions 1 to 11 supported
* Backfilling locally and via federation
* Accounts, profiles and devices
* Published room lists
* Typing
* Media APIs
* Redaction
* Tagging
* Context
* E2E keys and device lists
* Receipts
* Push
* Guests
* User Directory
* Presence
* Fulltext search

The primary goal of this fork is to make things simpler and easier to maintain. With that in mind, **a number of features have been removed**, including SQLite database support, the appservice API, support for 3PIDs, all P2P and relay-related work, support for the WebAssembly target, phone-home stats and others.

Aside from that, Harmony can largely operate as a drop-in replacement for Dendrite.

Harmony is Apache-2.0 licensed and will remain so.

## Documentation

For now, [Dendrite's documentation](https://matrix-org.github.io/dendrite/) is largely relevant. The configuration format and steps are practically unchanged.

## Future

This project may or may not have a long-term future. Let's see how things go.

If it does, then future changes are likely to include refactoring various parts of the codebase, swapping out some parts of the backend storage to make better use of NATS in places where it makes sense, as well as potentially reintroducing the ability to run some components standalone for sharding.
