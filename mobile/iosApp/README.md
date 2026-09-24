# iOS wrapper

Generate the Xcode project with `xcodegen generate` from this directory, then open `SitLab.xcodeproj`. The build phase links the simulator/device framework from the shared module. Set the Apple Team ID and provisioning profile before enabling production Universal Links; the repository intentionally does not publish `apple-app-site-association` until those external values are known.
