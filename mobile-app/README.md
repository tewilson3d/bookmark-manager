# Bookmarks iOS App

This is a Capacitor-wrapped version of the Bookmark Manager with iOS Share Extension support.

## Requirements

- Mac with Xcode 15+
- Apple Developer Account ($99/year for App Store, free for personal device testing)
- CocoaPods (`sudo gem install cocoapods`)

## Setup on Mac

1. **Copy this folder to your Mac**

2. **Install dependencies:**
   ```bash
   npm install
   ```

3. **Open in Xcode:**
   ```bash
   npx cap open ios
   ```

4. **Add the Share Extension to the Xcode project:**
   - In Xcode, select File → New → Target
   - Choose "Share Extension"
   - Name it "ShareExtension"
   - Replace the generated files with the ones in `ios/App/ShareExtension/`

5. **Configure App Groups (required for extension):**
   - Select the main App target → Signing & Capabilities
   - Add "App Groups" capability
   - Create a group like `group.xyz.exe.bookmarks`
   - Do the same for the ShareExtension target

6. **Update Bundle Identifiers:**
   - Main app: `xyz.exe.bookmarks`
   - Share Extension: `xyz.exe.bookmarks.ShareExtension`

7. **Build and run:**
   - Select your device or simulator
   - Press Cmd+R to build and run

## Share Extension Usage

Once installed, you can:
1. Open any app (Safari, LinkedIn, Instagram, etc.)
2. Tap the Share button
3. Select "Save to Bookmarks"
4. Add a note (optional) and tap Post

The bookmark will be saved to your server.

## API Configuration

The app connects to: `https://bookmark-manager.exe.xyz:8000`

To change this, edit `www/app.js` and update the `API_BASE` constant.

## Syncing Web Changes

After updating the web app:
```bash
npx cap copy ios
npx cap open ios
```
