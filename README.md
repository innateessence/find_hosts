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
