const DEFAULT_SERVER = 'https://bookmark-manager.exe.xyz:8000';

let serverUrl = DEFAULT_SERVER;

// Load settings
chrome.storage.sync.get(['serverUrl'], (result) => {
  if (result.serverUrl) {
    serverUrl = result.serverUrl;
    document.getElementById('server-url').value = serverUrl;
  }
});

// Get current tab info
chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
  const tab = tabs[0];
  const url = tab?.url || '';
  const title = tab?.title || '';

  document.getElementById('url').value = url;
  document.getElementById('title').value = title;

  // Detect source type
  const badge = document.getElementById('source-badge');
  if (url && url.includes('instagram.com')) {
    badge.textContent = 'Instagram';
    badge.className = 'source-badge source-instagram';
  } else if (url && url.includes('linkedin.com')) {
    badge.textContent = 'LinkedIn';
    badge.className = 'source-badge source-linkedin';
  } else {
    badge.textContent = 'Web';
    badge.className = 'source-badge source-web';
  }

  // Try to get description from page
  chrome.scripting.executeScript({
    target: { tabId: tab.id },
    func: () => {
      const meta = document.querySelector('meta[name="description"]') ||
                   document.querySelector('meta[property="og:description"]');
      return meta ? meta.content : '';
    }
  }).then((results) => {
    if (results && results[0] && results[0].result) {
      document.getElementById('description').value = results[0].result;
    }
  }).catch(() => {});
});

// Handle form submit
document.getElementById('bookmark-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  
  const saveBtn = document.getElementById('save-btn');
  const status = document.getElementById('status');
  
  saveBtn.disabled = true;
  saveBtn.textContent = 'Saving...';
  status.className = 'status hidden';

  const data = {
    url: document.getElementById('url').value,
    title: document.getElementById('title').value,
    description: document.getElementById('description').value,
    tags: document.getElementById('tags').value
      .split(',')
      .map(t => t.trim())
      .filter(Boolean)
  };

  try {
    const response = await fetch(`${serverUrl}/api/bookmarks`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    });

    if (response.ok) {
      status.textContent = '✓ Bookmark saved!';
      status.className = 'status success';
      saveBtn.textContent = 'Saved!';
      setTimeout(() => window.close(), 1500);
    } else {
      throw new Error('Failed to save');
    }
  } catch (err) {
    status.textContent = '✗ Error: Could not connect to server';
    status.className = 'status error';
    saveBtn.disabled = false;
    saveBtn.textContent = 'Save Bookmark';
  }
});

// Open app button
document.getElementById('open-app').addEventListener('click', () => {
  chrome.tabs.create({ url: serverUrl });
});

// Settings toggle
document.getElementById('settings-toggle').addEventListener('click', () => {
  const content = document.getElementById('settings-content');
  content.classList.toggle('hidden');
});

// Save settings
document.getElementById('save-settings').addEventListener('click', () => {
  const newUrl = document.getElementById('server-url').value.trim();
  if (newUrl) {
    serverUrl = newUrl;
    chrome.storage.sync.set({ serverUrl: newUrl }, () => {
      const status = document.getElementById('status');
      status.textContent = '✓ Settings saved!';
      status.className = 'status success';
      setTimeout(() => status.className = 'status hidden', 2000);
    });
  }
});
