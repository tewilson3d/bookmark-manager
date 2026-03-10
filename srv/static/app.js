    let currentSource = '';
    let debounceTimer;
    let selectionMode = false;
    let selectedBookmarks = new Set();
    
    function toggleSidebar() {
        const sidebar = document.getElementById('sidebar');
        const overlay = document.getElementById('sidebar-overlay');
        sidebar.classList.toggle('open');
        overlay.classList.toggle('open');
    }
    
    // Close sidebar when clicking a nav item on mobile
    function closeSidebarOnMobile() {
        if (window.innerWidth <= 768) {
            toggleSidebar();
        }
    }

    async function loadBookmarks(source = '') {
        currentSource = source;
        const grid = document.getElementById('bookmarks-grid');
        const loading = document.getElementById('loading');
        const empty = document.getElementById('empty-state');
        
        loading.classList.remove('hidden');
        grid.innerHTML = '';
        
        // "Things to Try" and "AI" show bookmarks by tag
        let url;
        if (source === 'things-to-try') {
            url = '/api/bookmarks?tag=try';
        } else if (source === 'ai') {
            url = '/api/bookmarks?tag=ai';
        } else if (source) {
            url = `/api/bookmarks?source=${source}`;
        } else {
            url = '/api/bookmarks';
        }
        const res = await fetch(url);
        const bookmarks = await res.json();
        
        loading.classList.add('hidden');
        
        if (!bookmarks || bookmarks.length === 0) {
            empty.classList.remove('hidden');
            return;
        }
        empty.classList.add('hidden');
        
        bookmarks.forEach(b => grid.appendChild(createCard(b)));
    }

    function createCard(b) {
        const card = document.createElement('div');
        card.className = 'bg-gray-800 rounded-lg overflow-hidden hover:ring-2 hover:ring-indigo-500 transition cursor-pointer bookmark-card';
        card.dataset.id = b.id;
        card.onclick = (e) => {
            if (selectionMode) {
                toggleCardSelection(card, b.id);
            } else {
                viewBookmark(b.id);
            }
        };
        
        const sourceClass = `source-${b.source_type}`;
        const sourceIcon = b.source_type === 'instagram' ? 'fab fa-instagram' : 
                          b.source_type === 'linkedin' ? 'fab fa-linkedin' : 
                          b.source_type === 'youtube' ? 'fab fa-youtube' : 
                          b.source_type === '3d' ? 'fas fa-cube' :
                          b.source_type === 'things-to-try' ? 'fas fa-flask' :
                          b.source_type === 'ai' ? 'fas fa-robot' : 'fas fa-globe';
        
        // Use favicon if available, otherwise show source icon
        const faviconHtml = b.favicon_url 
            ? `<img src="${b.favicon_url}" class="w-6 h-6 rounded" onerror="this.outerHTML='<span class=\\'${sourceClass} w-6 h-6 rounded flex items-center justify-center text-xs\\'><i class=\\'${sourceIcon}\\'></i></span>'">`
            : `<span class="${sourceClass} w-6 h-6 rounded flex items-center justify-center text-xs"><i class="${sourceIcon}"></i></span>`;
        
        card.innerHTML = `
            ${b.image_url ? `<img src="${b.image_url}" class="w-full h-32 object-cover" onerror="this.style.display='none'">` : ''}
            <div class="p-4 relative">
                <div class="flex items-start gap-2 mb-2">
                    ${faviconHtml}
                    <h3 class="font-medium line-clamp-2 flex-1 pr-6">${escapeHtml(b.title)}</h3>
                </div>
                ${b.description ? `<p class="text-gray-400 text-sm line-clamp-2 mb-2">${escapeHtml(b.description)}</p>` : ''}
                <div class="flex justify-between items-center">
                    <div class="text-gray-500 text-xs truncate">${new URL(b.url).hostname}</div>
                    <div class="relative" onclick="event.stopPropagation()">
                        <button onclick="toggleMenu(${b.id})" class="text-gray-400 hover:text-white p-1 rounded hover:bg-gray-700">
                            <i class="fas fa-ellipsis-v"></i>
                        </button>
                        <div id="menu-${b.id}" class="hidden absolute right-0 bottom-8 bg-gray-700 rounded-lg shadow-lg py-1 min-w-[120px] z-10">
                            <button onclick="openBookmark('${b.url}')" class="w-full text-left px-4 py-2 text-sm hover:bg-gray-600 flex items-center gap-2">
                                <i class="fas fa-external-link-alt"></i> Open
                            </button>
                            <button onclick="deleteBookmarkDirect(${b.id})" class="w-full text-left px-4 py-2 text-sm hover:bg-gray-600 text-red-400 flex items-center gap-2">
                                <i class="fas fa-trash"></i> Delete
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        `;
        return card;
    }

    async function viewBookmark(id) {
        const res = await fetch(`/api/bookmarks/${id}`);
        const data = await res.json();
        const b = data.bookmark;
        const tags = data.tags || [];
        
        // Parse keywords if stored as JSON
        let keywords = [];
        if (b.keywords) {
            try {
                keywords = JSON.parse(b.keywords);
            } catch(e) {
                keywords = b.keywords.split(',').map(k => k.trim());
            }
        }
        
        document.getElementById('view-content').innerHTML = `
            <div class="flex justify-between items-start mb-4">
                <h2 class="text-xl font-bold">${escapeHtml(b.title)}</h2>
                <button onclick="hideViewModal()" class="text-gray-400 hover:text-white">
                    <i class="fas fa-times"></i>
                </button>
            </div>
            ${b.image_url ? `<img src="${b.image_url}" class="w-full h-48 object-cover rounded-lg mb-4" onerror="this.style.display='none'">` : ''}
            <p class="text-gray-300 mb-4">${escapeHtml(b.description || '')}</p>
            
            <!-- Summary Section -->
            <div class="bg-gray-700 rounded-lg p-4 mb-4">
                <div class="flex justify-between items-center mb-2">
                    <strong><i class="fas fa-file-alt mr-2"></i>Summary</strong>
                    <button onclick="analyzeBookmark(${b.id})" class="text-sm bg-indigo-600 hover:bg-indigo-700 px-2 py-1 rounded" id="analyze-btn-${b.id}">
                        <i class="fas fa-magic"></i> ${b.summary ? 'Refresh' : 'Generate'}
                    </button>
                </div>
                <p id="summary-${b.id}" class="text-gray-300 text-sm">
                    ${b.summary ? escapeHtml(b.summary) : '<span class="text-gray-500 italic">No summary yet. Click Generate to analyze this page.</span>'}
                </p>
            </div>
            
            <!-- Keywords Section -->
            <div class="bg-gray-700 rounded-lg p-4 mb-4">
                <strong class="block mb-2"><i class="fas fa-tags mr-2"></i>Keywords</strong>
                <div id="keywords-${b.id}" class="flex flex-wrap gap-2">
                    ${keywords.length > 0 
                        ? keywords.map(k => `<span class="bg-gray-600 px-2 py-1 rounded text-xs">${escapeHtml(k)}</span>`).join('')
                        : '<span class="text-gray-500 italic text-sm">No keywords yet. Generate summary to extract keywords.</span>'
                    }
                </div>
            </div>
            
            <!-- Tags Section -->
            ${tags.length > 0 ? `
            <div class="mb-4">
                <strong class="block mb-2"><i class="fas fa-bookmark mr-2"></i>Tags</strong>
                <div class="flex flex-wrap gap-2">
                    ${tags.map(t => `<span class="bg-indigo-600 px-2 py-1 rounded text-sm">${escapeHtml(t.name)}</span>`).join('')}
                </div>
            </div>
            ` : ''}
            
            <div class="flex gap-2 pt-2 border-t border-gray-600 flex-wrap">
                <a href="${b.url}" target="_blank" class="bg-indigo-600 hover:bg-indigo-700 px-4 py-2 rounded flex items-center gap-2">
                    <i class="fas fa-external-link-alt"></i> Open
                </a>
                <button onclick="addToTry(${b.id})" class="bg-purple-600 hover:bg-purple-700 px-4 py-2 rounded flex items-center gap-2" id="try-btn-${b.id}">
                    <i class="fas fa-flask"></i> ${tags.some(t => t.name.toLowerCase() === 'try') ? 'Added!' : 'Try'}
                </button>
                ${tags.some(t => t.name.toLowerCase() === 'try') ? `
                <button onclick="finishTry(${b.id})" class="bg-green-600 hover:bg-green-700 px-4 py-2 rounded flex items-center gap-2" id="finish-btn-${b.id}">
                    <i class="fas fa-check"></i> Finished
                </button>` : ''}
                <button onclick="deleteBookmark(${b.id})" class="bg-red-600 hover:bg-red-700 px-4 py-2 rounded">
                    <i class="fas fa-trash"></i>
                </button>
            </div>
        `;
        document.getElementById('view-modal').classList.remove('hidden');
        document.getElementById('view-modal').classList.add('flex');
    }

    async function analyzeBookmark(id) {
        const btn = document.getElementById(`analyze-btn-${id}`);
        const summaryEl = document.getElementById(`summary-${id}`);
        const keywordsEl = document.getElementById(`keywords-${id}`);
        
        btn.disabled = true;
        btn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Analyzing...';
        summaryEl.innerHTML = '<span class="text-gray-400"><i class="fas fa-spinner fa-spin"></i> Fetching and analyzing content...</span>';
        
        try {
            const res = await fetch(`/api/bookmarks/${id}/analyze`, { method: 'POST' });
            const data = await res.json();
            
            if (data.error) {
                summaryEl.innerHTML = `<span class="text-red-400">${escapeHtml(data.error)}</span>`;
            } else {
                summaryEl.innerHTML = escapeHtml(data.bookmark.summary || 'No summary could be generated.');
                
                const keywords = data.keywords || [];
                keywordsEl.innerHTML = keywords.length > 0
                    ? keywords.map(k => `<span class="bg-gray-600 px-2 py-1 rounded text-xs">${escapeHtml(k)}</span>`).join('')
                    : '<span class="text-gray-500 italic text-sm">No keywords found.</span>';
            }
        } catch (err) {
            summaryEl.innerHTML = '<span class="text-red-400">Failed to analyze. The page may be inaccessible.</span>';
        }
        
        btn.disabled = false;
        btn.innerHTML = '<i class="fas fa-magic"></i> Refresh';
    }

    async function deleteBookmark(id) {
        if (!confirm('Delete this bookmark?')) return;
        await fetch(`/api/bookmarks/${id}`, { method: 'DELETE' });
        hideViewModal();
        loadBookmarks(currentSource);
    }

    async function addToTry(id) {
        const btn = document.getElementById(`try-btn-${id}`);
        btn.disabled = true;
        btn.innerHTML = '<i class="fas fa-spinner fa-spin"></i>';
        
        try {
            await fetch(`/api/bookmarks/${id}/tag/try`, { method: 'POST' });
            btn.innerHTML = '<i class="fas fa-check"></i> Added!';
            btn.classList.remove('bg-purple-600', 'hover:bg-purple-700');
            btn.classList.add('bg-green-600');
        } catch(e) {
            btn.innerHTML = '<i class="fas fa-times"></i> Error';
            btn.disabled = false;
        }
    }

    async function finishTry(id) {
        const btn = document.getElementById(`finish-btn-${id}`);
        btn.disabled = true;
        btn.innerHTML = '<i class="fas fa-spinner fa-spin"></i>';
        
        try {
            await fetch(`/api/bookmarks/${id}/tag/try`, { method: 'DELETE' });
            btn.innerHTML = '<i class="fas fa-check"></i> Done!';
            // Close modal and refresh if we're in Things to Try view
            setTimeout(() => {
                hideViewModal();
                loadBookmarks(currentSource);
            }, 500);
        } catch(e) {
            btn.innerHTML = '<i class="fas fa-times"></i> Error';
            btn.disabled = false;
        }
    }

    async function deleteBookmarkDirect(id) {
        if (!confirm('Delete this bookmark?')) return;
        await fetch(`/api/bookmarks/${id}`, { method: 'DELETE' });
        loadBookmarks(currentSource);
    }

    function toggleMenu(id) {
        // Close all other menus first
        document.querySelectorAll('[id^="menu-"]').forEach(menu => {
            if (menu.id !== `menu-${id}`) {
                menu.classList.add('hidden');
            }
        });
        // Toggle this menu
        const menu = document.getElementById(`menu-${id}`);
        menu.classList.toggle('hidden');
    }

    function openBookmark(url) {
        window.open(url, '_blank');
    }

    // Close menus when clicking outside
    document.addEventListener('click', (e) => {
        if (!e.target.closest('[id^="menu-"]') && !e.target.closest('button[onclick^="toggleMenu"]')) {
            document.querySelectorAll('[id^="menu-"]').forEach(menu => menu.classList.add('hidden'));
        }
    });

    function showAddModal() {
        document.getElementById('add-modal').classList.remove('hidden');
        document.getElementById('add-modal').classList.add('flex');
    }

    function hideAddModal() {
        document.getElementById('add-modal').classList.add('hidden');
        document.getElementById('add-modal').classList.remove('flex');
        document.getElementById('add-form').reset();
    }

    function hideViewModal() {
        document.getElementById('view-modal').classList.add('hidden');
        document.getElementById('view-modal').classList.remove('flex');
    }

    async function fetchMeta() {
        const url = document.querySelector('input[name="url"]').value;
        if (!url) return;
        
        const res = await fetch('/api/fetch-metadata', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url })
        });
        const meta = await res.json();
        
        if (meta.title) document.querySelector('input[name="title"]').value = meta.title;
        if (meta.description) document.querySelector('textarea[name="description"]').value = meta.description;
    }

    document.getElementById('add-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const form = e.target;
        const data = {
            url: form.url.value,
            title: form.title.value,
            description: form.description.value,
            tags: form.tags.value.split(',').map(t => t.trim()).filter(Boolean)
        };
        
        await fetch('/api/bookmarks', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        
        hideAddModal();
        loadBookmarks(currentSource);
        loadTags();
    });

    document.getElementById('search-input').addEventListener('input', (e) => {
        clearTimeout(debounceTimer);
        debounceTimer = setTimeout(() => searchBookmarks(e.target.value), 300);
    });

    async function searchBookmarks(query) {
        if (!query) {
            loadBookmarks(currentSource);
            return;
        }
        
        const grid = document.getElementById('bookmarks-grid');
        const res = await fetch(`/api/search?q=${encodeURIComponent(query)}`);
        const data = await res.json();
        
        grid.innerHTML = '';
        (data.bookmarks || []).forEach(b => grid.appendChild(createCard(b)));
    }

    async function loadTags() {
        const res = await fetch('/api/tags');
        const tags = await res.json();
        const container = document.getElementById('tags-list');
        container.innerHTML = (tags || []).map(t => 
            `<span class="bg-gray-700 px-2 py-1 rounded text-xs cursor-pointer hover:bg-gray-600">${escapeHtml(t.name)}</span>`
        ).join('');
    }

    async function loadCollections() {
        const res = await fetch('/api/collections');
        const collections = await res.json();
        const container = document.getElementById('collections-list');
        container.innerHTML = (collections || []).map(c => 
            `<button onclick="loadCollectionBookmarks(${c.id})" class="w-full text-left px-3 py-2 rounded hover:bg-gray-700 flex items-center gap-2">
                ${c.icon || '📁'} ${escapeHtml(c.name)}
            </button>`
        ).join('');
    }
    
    async function loadCollectionBookmarks(collectionId) {
        currentSource = '';
        const grid = document.getElementById('bookmarks-grid');
        const loading = document.getElementById('loading');
        const empty = document.getElementById('empty-state');
        
        loading.classList.remove('hidden');
        grid.innerHTML = '';
        
        const res = await fetch(`/api/collections/${collectionId}/bookmarks`);
        const bookmarks = await res.json();
        
        loading.classList.add('hidden');
        
        if (!bookmarks || bookmarks.length === 0) {
            empty.classList.remove('hidden');
            return;
        }
        empty.classList.add('hidden');
        
        bookmarks.forEach(b => grid.appendChild(createCard(b)));
    }

    function escapeHtml(str) {
        if (!str) return '';
        return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    }

    function showWebSearchModal() {
        document.getElementById('web-search-modal').classList.remove('hidden');
        document.getElementById('web-search-modal').classList.add('flex');
        document.getElementById('web-search-input').focus();
    }

    function hideWebSearchModal() {
        document.getElementById('web-search-modal').classList.add('hidden');
        document.getElementById('web-search-modal').classList.remove('flex');
    }

    async function doWebSearch() {
        const query = document.getElementById('web-search-input').value;
        if (!query) return;
        
        const resultsDiv = document.getElementById('web-search-results');
        resultsDiv.innerHTML = '<div class="text-center py-4"><i class="fas fa-spinner fa-spin"></i> Searching...</div>';
        
        const res = await fetch(`/api/web-search?q=${encodeURIComponent(query)}`);
        const data = await res.json();
        
        if (!data.results || data.results.length === 0) {
            resultsDiv.innerHTML = `
                <p class="text-gray-400">No instant results. Try searching directly:</p>
                <a href="${data.search_url}" target="_blank" class="text-indigo-400 hover:underline">Search on DuckDuckGo</a>
            `;
            return;
        }
        
        resultsDiv.innerHTML = data.results.map(r => `
            <div class="bg-gray-700 rounded-lg p-4 hover:bg-gray-600">
                <h3 class="font-medium mb-1">${escapeHtml(r.title)}</h3>
                ${r.description ? `<p class="text-gray-400 text-sm mb-2">${escapeHtml(r.description)}</p>` : ''}
                <div class="flex gap-2">
                    <a href="${r.url}" target="_blank" class="text-indigo-400 text-sm hover:underline">
                        <i class="fas fa-external-link-alt"></i> Visit
                    </a>
                    <button onclick="quickSaveBookmark('${escapeHtml(r.url)}', '${escapeHtml(r.title)}', '${escapeHtml(r.description || '')}')" 
                        class="text-green-400 text-sm hover:underline">
                        <i class="fas fa-bookmark"></i> Save
                    </button>
                </div>
            </div>
        `).join('') + `
            <div class="mt-4 text-center">
                <a href="${data.search_url}" target="_blank" class="text-indigo-400 hover:underline">More results on DuckDuckGo</a>
            </div>
        `;
    }

    async function quickSaveBookmark(url, title, description) {
        await fetch('/api/bookmarks', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url, title, description })
        });
        alert('Bookmark saved!');
        loadBookmarks(currentSource);
    }

    document.getElementById('web-search-input').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') doWebSearch();
    });

    // GitHub Integration
    function loadGitHubConfig() {
        fetch('/api/github/config').then(r => r.json()).then(data => {
            if (data.repo) document.getElementById('github-repo').value = data.repo;
            if (data.token) document.getElementById('github-token').value = data.token;
            if (data.branch) document.getElementById('github-branch').value = data.branch;
        }).catch(() => {});
    }

    async function saveGitHubConfig() {
        const repo = document.getElementById('github-repo').value.trim();
        const token = document.getElementById('github-token').value.trim();
        const branch = document.getElementById('github-branch').value.trim() || 'main';
        const statusDiv = document.getElementById('github-status');
        
        if (!repo) {
            alert('Please enter a repository URL');
            return;
        }
        
        statusDiv.className = 'text-sm bg-gray-800 p-2 rounded';
        statusDiv.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Saving configuration...';
        
        try {
            const res = await fetch('/api/github/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ repo, token, branch })
            });
            const data = await res.json();
            
            if (data.error) {
                statusDiv.className = 'text-sm bg-red-900 p-2 rounded';
                statusDiv.innerHTML = '<i class="fas fa-times"></i> ' + data.error;
            } else {
                statusDiv.className = 'text-sm bg-green-900 p-2 rounded';
                statusDiv.innerHTML = '<i class="fas fa-check"></i> Configuration saved!';
            }
        } catch (err) {
            statusDiv.className = 'text-sm bg-red-900 p-2 rounded';
            statusDiv.innerHTML = '<i class="fas fa-times"></i> Error saving configuration';
        }
    }

    async function gitPull() {
        const statusDiv = document.getElementById('github-status');
        statusDiv.className = 'text-sm bg-gray-800 p-2 rounded';
        statusDiv.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Pulling from GitHub...';
        
        try {
            const res = await fetch('/api/github/pull', { method: 'POST' });
            const data = await res.json();
            
            if (data.error) {
                statusDiv.className = 'text-sm bg-red-900 p-2 rounded';
                statusDiv.innerHTML = '<i class="fas fa-times"></i> ' + data.error;
            } else {
                statusDiv.className = 'text-sm bg-green-900 p-2 rounded';
                statusDiv.innerHTML = '<i class="fas fa-check"></i> ' + (data.message || 'Pull successful!');
            }
        } catch (err) {
            statusDiv.className = 'text-sm bg-red-900 p-2 rounded';
            statusDiv.innerHTML = '<i class="fas fa-times"></i> Error pulling from GitHub';
        }
    }

    async function gitPush() {
        const statusDiv = document.getElementById('github-status');
        const message = prompt('Enter commit message:', 'Update bookmark manager');
        if (!message) return;
        
        statusDiv.className = 'text-sm bg-gray-800 p-2 rounded';
        statusDiv.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Pushing to GitHub...';
        
        try {
            const res = await fetch('/api/github/push', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ message })
            });
            const data = await res.json();
            
            if (data.error) {
                statusDiv.className = 'text-sm bg-red-900 p-2 rounded';
                statusDiv.innerHTML = '<i class="fas fa-times"></i> ' + data.error;
            } else {
                statusDiv.className = 'text-sm bg-green-900 p-2 rounded';
                statusDiv.innerHTML = '<i class="fas fa-check"></i> ' + (data.message || 'Push successful!');
            }
        } catch (err) {
            statusDiv.className = 'text-sm bg-red-900 p-2 rounded';
            statusDiv.innerHTML = '<i class="fas fa-times"></i> Error pushing to GitHub';
        }
    }

    // Load GitHub config when tools modal opens
    const origShowToolsModal = showToolsModal;
    showToolsModal = function() {
        origShowToolsModal ? origShowToolsModal() : null;
        document.getElementById('tools-modal').classList.remove('hidden');
        document.getElementById('tools-modal').classList.add('flex');
        loadGitHubConfig();
    };

    async function removeDuplicates() {
        const btn = document.getElementById('remove-dupes-btn');
        const statusDiv = document.getElementById('remove-dupes-status');
        
        if (!confirm('This will remove all duplicate bookmarks (same URL), keeping the oldest one. Continue?')) {
            return;
        }
        
        btn.disabled = true;
        btn.innerHTML = '<i class="fas fa-spinner fa-spin mr-1"></i> Processing...';
        statusDiv.className = 'text-sm mt-2 bg-gray-800 p-2 rounded';
        statusDiv.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Finding and removing duplicates...';
        
        try {
            const res = await fetch('/api/remove-duplicates', { method: 'POST' });
            const data = await res.json();
            
            if (data.error) {
                statusDiv.className = 'text-sm mt-2 bg-red-900 p-2 rounded';
                statusDiv.innerHTML = '<i class="fas fa-times"></i> ' + data.error;
            } else {
                statusDiv.className = 'text-sm mt-2 bg-green-900 p-2 rounded';
                statusDiv.innerHTML = `<i class="fas fa-check"></i> Done! Removed ${data.deleted} duplicate bookmarks.`;
                loadBookmarks(currentSource);
            }
        } catch (err) {
            statusDiv.className = 'text-sm mt-2 bg-red-900 p-2 rounded';
            statusDiv.innerHTML = '<i class="fas fa-times"></i> Error removing duplicates';
        }
        
        btn.disabled = false;
        btn.innerHTML = '<i class="fas fa-broom mr-1"></i> Remove Duplicates';
    }

    async function generateAllMetadata() {
        const btn = document.getElementById('generate-all-btn');
        const statusDiv = document.getElementById('generate-all-status');
        
        btn.disabled = true;
        btn.innerHTML = '<i class="fas fa-spinner fa-spin mr-1"></i> Processing...';
        statusDiv.className = 'text-sm mt-2 bg-gray-800 p-2 rounded';
        statusDiv.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Generating metadata for all bookmarks... This may take a while.';
        
        try {
            const res = await fetch('/api/generate-all', { method: 'POST' });
            const data = await res.json();
            
            if (data.error) {
                statusDiv.className = 'text-sm mt-2 bg-red-900 p-2 rounded';
                statusDiv.innerHTML = '<i class="fas fa-times"></i> ' + data.error;
            } else {
                statusDiv.className = 'text-sm mt-2 bg-green-900 p-2 rounded';
                statusDiv.innerHTML = `<i class="fas fa-check"></i> Done! Updated ${data.updated} of ${data.total} bookmarks.`;
                loadBookmarks(currentSource);
            }
        } catch (err) {
            statusDiv.className = 'text-sm mt-2 bg-red-900 p-2 rounded';
            statusDiv.innerHTML = '<i class="fas fa-times"></i> Error generating metadata';
        }
        
        btn.disabled = false;
        btn.innerHTML = '<i class="fas fa-wand-magic-sparkles mr-1"></i> Generate All';
    }

    async function importInstagramData() {
        const fileInput = document.getElementById('instagram-file');
        const statusDiv = document.getElementById('instagram-import-status');
        
        if (!fileInput.files || !fileInput.files[0]) {
            alert('Please select a JSON file');
            return;
        }
        
        statusDiv.className = 'text-sm bg-gray-800 p-2 rounded';
        statusDiv.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Importing...';
        
        const formData = new FormData();
        formData.append('file', fileInput.files[0]);
        
        try {
            const res = await fetch('/api/instagram/import', {
                method: 'POST',
                body: formData
            });
            
            const data = await res.json();
            
            if (data.error) {
                statusDiv.className = 'text-sm bg-red-900 p-2 rounded';
                statusDiv.innerHTML = '<i class="fas fa-times"></i> ' + data.error;
            } else {
                statusDiv.className = 'text-sm bg-green-900 p-2 rounded';
                statusDiv.innerHTML = `<i class="fas fa-check"></i> Imported ${data.saved} posts (${data.skipped} already existed)`;
                fileInput.value = '';
                loadBookmarks('instagram');
            }
        } catch (err) {
            statusDiv.className = 'text-sm bg-red-900 p-2 rounded';
            statusDiv.innerHTML = '<i class="fas fa-times"></i> Error importing file';
        }
    }

    async function importYouTubePlaylist() {
        const urlInput = document.getElementById('youtube-playlist-url');
        const apiKeyInput = document.getElementById('youtube-api-key');
        const statusDiv = document.getElementById('youtube-import-status');
        
        const playlistUrl = urlInput.value.trim();
        if (!playlistUrl) {
            alert('Please enter a YouTube playlist URL');
            return;
        }
        
        statusDiv.className = 'text-sm bg-gray-800 p-2 rounded';
        statusDiv.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Importing playlist...';
        
        try {
            const res = await fetch('/api/youtube/import', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    playlist_url: playlistUrl,
                    api_key: apiKeyInput.value.trim()
                })
            });
            
            const data = await res.json();
            
            if (data.error) {
                statusDiv.className = 'text-sm bg-red-900 p-2 rounded';
                statusDiv.innerHTML = '<i class="fas fa-times"></i> ' + data.error;
            } else {
                statusDiv.className = 'text-sm bg-green-900 p-2 rounded';
                statusDiv.innerHTML = `<i class="fas fa-check"></i> Imported ${data.saved} videos (${data.skipped} already existed)`;
                urlInput.value = '';
                loadBookmarks('youtube');
            }
        } catch (err) {
            statusDiv.className = 'text-sm bg-red-900 p-2 rounded';
            statusDiv.innerHTML = '<i class="fas fa-times"></i> Error importing playlist';
        }
    }

    function showToolsModal() {
        document.getElementById('tools-modal').classList.remove('hidden');
        document.getElementById('tools-modal').classList.add('flex');
    }

    function hideToolsModal() {
        document.getElementById('tools-modal').classList.add('hidden');
        document.getElementById('tools-modal').classList.remove('flex');
    }

    async function quickAddURL() {
        const urlInput = document.getElementById('quick-url');
        const url = urlInput.value.trim();
        if (!url) return;
        
        // Fetch metadata first
        try {
            const metaRes = await fetch('/api/fetch-metadata', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ url })
            });
            const meta = await metaRes.json();
            
            await fetch('/api/bookmarks', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    url: url,
                    title: meta.title || url,
                    description: meta.description || '',
                    source_type: meta.source_type || 'web'
                })
            });
            
            urlInput.value = '';
            hideToolsModal();
            loadBookmarks(currentSource);
            alert('Bookmark saved!');
        } catch (err) {
            alert('Error saving bookmark');
        }
    }

    // Check for save parameter (from bookmarklet)
    const urlParams = new URLSearchParams(window.location.search);
    if (urlParams.get('save')) {
        const saveUrl = urlParams.get('save');
        const saveTitle = urlParams.get('title') || '';
        // Open add modal with pre-filled data
        showAddModal();
        document.querySelector('input[name="url"]').value = saveUrl;
        document.querySelector('input[name="title"]').value = saveTitle;
        fetchMeta();
        // Clean URL
        history.replaceState({}, '', '/');
    }

    // Selection mode functions
    function toggleSelectionMode() {
        selectionMode = !selectionMode;
        selectedBookmarks.clear();
        updateSelectionUI();
        
        const grid = document.getElementById('bookmarks-grid');
        const toolbar = document.getElementById('selection-toolbar');
        const selectBtn = document.getElementById('select-btn');
        
        if (selectionMode) {
            grid.classList.add('selection-mode');
            toolbar.classList.remove('hidden');
            toolbar.classList.add('flex');
            selectBtn.classList.add('bg-indigo-600');
            selectBtn.classList.remove('bg-gray-700');
            loadCollectionsForDropdown();
        } else {
            grid.classList.remove('selection-mode');
            toolbar.classList.add('hidden');
            toolbar.classList.remove('flex');
            selectBtn.classList.remove('bg-indigo-600');
            selectBtn.classList.add('bg-gray-700');
            document.querySelectorAll('.bookmark-card.selected').forEach(c => c.classList.remove('selected'));
        }
    }
    
    function toggleCardSelection(card, id) {
        if (selectedBookmarks.has(id)) {
            selectedBookmarks.delete(id);
            card.classList.remove('selected');
        } else {
            selectedBookmarks.add(id);
            card.classList.add('selected');
        }
        updateSelectionUI();
    }
    
    function updateSelectionUI() {
        document.getElementById('selection-count').textContent = `${selectedBookmarks.size} selected`;
    }
    
    async function loadCollectionsForDropdown() {
        const res = await fetch('/api/collections');
        const collections = await res.json();
        const select = document.getElementById('add-to-collection');
        select.innerHTML = '<option value="">Add to collection...</option>';
        (collections || []).forEach(c => {
            select.innerHTML += `<option value="${c.id}">${c.icon || '📁'} ${escapeHtml(c.name)}</option>`;
        });
    }
    
    async function moveSelectedToCategory() {
        const select = document.getElementById('move-to-category');
        const category = select.value;
        if (!category || selectedBookmarks.size === 0) return;
        
        await fetch('/api/bookmarks/bulk-update', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                bookmark_ids: Array.from(selectedBookmarks),
                source_type: category
            })
        });
        
        select.value = '';
        toggleSelectionMode();
        loadBookmarks(currentSource);
    }
    
    async function addSelectedToCollection() {
        const select = document.getElementById('add-to-collection');
        const collectionId = select.value;
        if (!collectionId || selectedBookmarks.size === 0) return;
        
        await fetch(`/api/collections/${collectionId}/bookmarks`, {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                bookmark_ids: Array.from(selectedBookmarks)
            })
        });
        
        select.value = '';
        toggleSelectionMode();
        loadCollections();
    }
    
    async function deleteSelected() {
        if (selectedBookmarks.size === 0) return;
        if (!confirm(`Delete ${selectedBookmarks.size} bookmarks?`)) return;
        
        for (const id of selectedBookmarks) {
            await fetch(`/api/bookmarks/${id}`, {method: 'DELETE'});
        }
        
        toggleSelectionMode();
        loadBookmarks(currentSource);
    }
    
    function showCreateCollectionModal() {
        const name = prompt('Collection name:');
        if (!name) return;
        const icon = prompt('Icon (emoji):', '📁') || '📁';
        
        fetch('/api/collections', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({name, icon})
        }).then(() => loadCollections());
    }

    // Initial load
    loadBookmarks();
    loadTags();
    loadCollections();
    
    // Register service worker for PWA and share target
    if ('serviceWorker' in navigator) {
        navigator.serviceWorker.register('/sw.js')
            .then(reg => console.log('Service worker registered:', reg.scope))
            .catch(err => console.error('Service worker registration failed:', err));
    }
