import UIKit
import Social
import MobileCoreServices
import UniformTypeIdentifiers

class ShareViewController: SLComposeServiceViewController {
    
    private var sharedURL: String?
    private var sharedTitle: String?
    
    override func isContentValid() -> Bool {
        return sharedURL != nil
    }
    
    override func viewDidLoad() {
        super.viewDidLoad()
        
        // Extract shared content
        if let extensionItem = extensionContext?.inputItems.first as? NSExtensionItem {
            for attachment in extensionItem.attachments ?? [] {
                
                // Handle URLs
                if attachment.hasItemConformingToTypeIdentifier(UTType.url.identifier) {
                    attachment.loadItem(forTypeIdentifier: UTType.url.identifier, options: nil) { [weak self] (data, error) in
                        if let url = data as? URL {
                            DispatchQueue.main.async {
                                self?.sharedURL = url.absoluteString
                                self?.validateContent()
                            }
                        }
                    }
                }
                
                // Handle plain text (might be a URL)
                else if attachment.hasItemConformingToTypeIdentifier(UTType.plainText.identifier) {
                    attachment.loadItem(forTypeIdentifier: UTType.plainText.identifier, options: nil) { [weak self] (data, error) in
                        if let text = data as? String {
                            DispatchQueue.main.async {
                                // Check if text contains a URL
                                if let url = self?.extractURL(from: text) {
                                    self?.sharedURL = url
                                } else {
                                    self?.sharedURL = text
                                }
                                self?.validateContent()
                            }
                        }
                    }
                }
            }
        }
    }
    
    private func extractURL(from text: String) -> String? {
        let detector = try? NSDataDetector(types: NSTextCheckingResult.CheckingType.link.rawValue)
        let matches = detector?.matches(in: text, options: [], range: NSRange(location: 0, length: text.utf16.count))
        
        if let match = matches?.first, let range = Range(match.range, in: text) {
            return String(text[range])
        }
        return nil
    }
    
    override func didSelectPost() {
        guard let url = sharedURL else {
            self.extensionContext?.completeRequest(returningItems: [], completionHandler: nil)
            return
        }
        
        // Send to our API
        let apiURL = URL(string: "https://bookmark-manager.exe.xyz:8000/api/bookmarks")!
        var request = URLRequest(url: apiURL)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        
        let title = contentText ?? "Shared from iOS"
        let body: [String: Any] = [
            "url": url,
            "title": title,
            "source_type": "web"
        ]
        
        request.httpBody = try? JSONSerialization.data(withJSONObject: body)
        
        let task = URLSession.shared.dataTask(with: request) { [weak self] data, response, error in
            DispatchQueue.main.async {
                self?.extensionContext?.completeRequest(returningItems: [], completionHandler: nil)
            }
        }
        task.resume()
    }
    
    override func configurationItems() -> [Any]! {
        return []
    }
}
