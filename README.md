# find_hosts

## What is this?

* This is a single go 'script' which locates all the devices that reply to ICMP packets on your local network
* Most devices reply to these packets by default unless you or a sys admin explicitly configures the device not too.
* It also tends to do this within less than a second. Should take roughly 500ms on average based on my tests.

In other words, this finds most of the devices in your local network that have an IP address

## Why?

* Sometimes I'm setting up a rasberry pi and I don't want to deal with hooking it up to a screen or keyboard or anything like that.
* ISP Foo wants me to use an android app to find things on my local network, which I find annoying. Their built-in router page isn't great either.
* I wanted to get my feet wet with writing `go` code and how `go` handles concurrency. It's nice.
* `ping` doesn't really handle the use-case of checking your whole network very well, so...
* I enjoy having as few 3rd party dependencies as possible
* You learn new things by writing your own tools, and it's a good choice whenever you have the allowance for mistakes to be made.

## Running this flooded my network with roughly 2,5000 ICMP packets

* This isn't really a big deal. But you're right. It's non optimal.
* It appears that when you perform a `ping` concurrently, you get packets in unexpected places.
* I wrote this in less than a day, having next to no previous golang experience.
* I solved this in a hacky way that has an extremely low false negative rate, but the cost is it sends out 10 requests for every possible IP in your local network.
* Thism means it will send 256 * 10 requests and attempt to identify the real sender of the packet

Disclaimer: I'm not a networking guy and hacked this together very quickly.

Disclaimer2: I'm not a go-lang guy, and it's very possible I'm making mistakes with concurrency here.
