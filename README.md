# Go Sts

So, TLDR first: I was using a fantastic tool for managing my credentials for the many many roles on AWS but a weird
set of commands made it unusable on my machine so I decided to rewrite it in Go. 

My experience in Go is close to 0 so I thought "what could go wrong if I try to replicate [this](https://github.com/ruimarinho/gsts) project"?

I replaced Playwright with the fantastic [chromedp](https://github.com/chromedp/chromedp) module.

Roadmap
- [x] First working implementation to have my credentials back
- [ ] Write usage docs
- [ ] GH Releases
- [ ] Implement tests
- [ ] Write more go compliant code (especially for error handling)
- [ ] Implement a docker headless version

Any comment,suggestion or feedback are more then welcome

