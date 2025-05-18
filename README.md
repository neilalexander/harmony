# Unmaintained

For a number of years I worked on Matrix and Dendrite, believing that the ecosystem would be enriched by a powerful and lightweight homeserver implementation to
live alongside or eventually replace Synapse. I am proud of the huge progress that the Dendrite team made in transforming an abandoned codebase that barely compiled
into a modernised and functional one. The Harmony fork came into existence with the thought that it's still possible to have a perfectly good core Matrix experience
without much of the extra weight that had accumulated over time, hence the number of features and APIs that had been removed, and that a leaner codebase would be overall
easier to maintain as an individual developer.

However, I have found myself increasingly disenfranchised. Matrix has become a perfect example of what happens when an endlessly scope-creeping monolithic specification
is combined with a complete lack of interest in repaying technical debt or fixing fundamental protocol issues. The result over time is that the protocol has become
extremely heavy, practically impossible to implement correctly (by whatever definition of the word), deeply flawed and, worst of all for a federated system, a complete
interoperability and compatibility disaster.

It's practically impossible to "get Matrix right" today, given the difficulty in reproducing the many Synapse bugs and the sheer lack of precision in the spec (which
some vocal community members use as an excuse to beat us endlessly with "Dendrite is unusable!"-style criticisms), and with the recent wave of illegal and frankly
horrifying content that has been spammed throughout and eagerly replicated across the Matrix federation, I am no longer convinced that a Matrix homeserver is even safe
to host, or indeed that anything remotely like this is even what "right" is *supposed* to look like.

I just no longer believe that Matrix is the right answer to the problems that it attempts to solve and with that I no longer have the energy or motivation to maintain
this fork either. Perhaps Dendrite development will continue upstream or someone else will fork Harmony, but time has come for me to say goodbye to this project and to
refocus my attention elsewhere.

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

The primary goal of this fork is to make things simpler and easier to maintain. With that in mind, **a number of features have been removed**, including SQLite database support, the appservice API, support for 3PIDs, support for OpenID, all P2P and relay-related work, support for the WebAssembly target, phone-home stats and others.

Aside from that, Harmony can largely operate as a drop-in replacement for Dendrite.

Harmony is Apache-2.0 licensed and will remain so.

## Documentation

For now, [Dendrite's documentation](https://matrix-org.github.io/dendrite/) is largely relevant. The configuration format and steps are practically unchanged.

## Future

This project may or may not have a long-term future. Let's see how things go.

If it does, then future changes are likely to include refactoring various parts of the codebase, swapping out some parts of the backend storage to make better use of NATS in places where it makes sense, as well as potentially reintroducing the ability to run some components standalone for sharding.
