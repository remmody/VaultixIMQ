import Alpine from 'alpinejs';
import './style.css';
import {
    GetAccounts,
    GetAccountsLight,
    GetAccountDetails,
    SetVisibleAccounts,
    AddAccount,
    DeleteAccount,
    GetTOTPList,
    AddTOTP,
    DeleteTOTP,
    GenerateTOTP,
    FetchEmails,
    FetchBody,
    GetSettings,
    UpdateSettings,
    GetNextSync,
    GetCachedMessages,
    ExportProfile,
    ImportProfile,
    SelectSavePath,
    SelectOpenPath,
    MarkAsRead,
    MarkAllAsRead,
    IsLocked,
    IsPasswordSet,
    NeedsSetup,
    VerifyAppPassword,
    SetAppPassword,
    SkipAppPasswordSetup,
    UnlockApp,
    GetAboutInfo,
    CheckForUpdates,
    StartEngine,
    IsVaultSet,
    IsUnsecuredImport,
    LockApp,
    LockVault
} from '../wailsjs/go/app/App';
import * as runtime from '../wailsjs/runtime';

globalThis.Alpine = Alpine;

// console.log("VaultixIMQ main.js loading...");

document.addEventListener('alpine:init', () => {
    // console.log("Alpine initialized, registering vaultixApp...");
    Alpine.data('vaultixApp', () => ({
        accounts: [],
        sortedAccounts: [],
        selectedAccount: null,
        totpList: [],
        messages: [],
        selectedMessage: null,
        messageBody: '',
        bodyLoading: false,
        sidebarMode: 'accounts',
        timeLeft: 30,
        refreshing: false,
        showAddAccount: false,
        showAddTOTP: false,
        showExportModal: false,
        showImportModal: false,
        showSettings: false,
        showAbout: false,
        showBulkAdd: false,
        showChangePassword: false,
        accountSearch: '',
        currentFolder: 'INBOX',
        changePassOld: '',
        changePassNew: '',
        changePassConfirm: '',
        passwordSet: false,
        totpTimers: {},
        
        // Virtual Scrolling State
        accountScrollTop: 0,
        accountViewportHeight: 600,
        accountItemHeight: 60,
        accountBuffer: 10,

        isLocked: false,
        showPasswordSetup: false,
        isUnsecuredImport: false,
        lockPass: '',
        setupPass: '',
        lastActivity: Date.now(),

        exportPass: '',
        importPass: '',
        exportPath: 'VaultixIMQ_backup.vaultix',
        importPath: '',
        newAcc: { email: '', password: '', host: '', port: '', label: '' },
        newTotp: { label: '', secret: '', issuer: '', account: '' },
        toast: { show: false, msg: '', title: '', email: '' },
        dialog: { show: false, title: '', message: '', type: 'alert', resolve: null },

        aboutInfo: {
            version: '1.0.0',
            author: 'RemmoDY',
            license: 'MIT License',
            github: 'https://github.com/remmody/VaultixIMQ'
        },
        updateInfo: {
            has_update: false,
            latest_version: '',
            download_url: ''
        },

        settings: {
            sync_interval: 60,
            auto_login: true,
            notifications: true,
            sound: true,
            auto_lock_interval: 15
        },
        nextSyncSecs: 0,
        bulkImapServer: '',
        notificationSound: new Audio('/notifications.wav'),

        openURL(url) {
            if (url) runtime.BrowserOpenURL(url);
        },

        async init() {
            // console.log("vaultixApp.init() starting...");
            try {
                // Ensure context is available
                if (!globalThis.go && !runtime) {
                    throw new Error("Wails context not found");
                }
                await this.loadSettings();
                this.isLocked = await IsLocked();
                this.showPasswordSetup = await NeedsSetup();
                this.isUnsecuredImport = await IsUnsecuredImport();

                // Consolidated Inactivity Tracker (Seconds-based from Settings)
                this.lastActivity = Date.now();
                setInterval(async () => {
                    if (!this.isLocked && this.settings.auto_lock_interval > 0) {
                        const inactiveSeconds = (Date.now() - this.lastActivity) / 1000;
                        if (inactiveSeconds >= this.settings.auto_lock_interval) {
                            if (await IsPasswordSet()) {
                                await LockApp();
                                this.isLocked = true;
                                this.showToast("Application locked due to inactivity");
                            }
                        }
                    }
                }, 1000); 

                ['mousedown', 'keydown', 'mousemove', 'touchstart'].forEach(evt => {
                    window.addEventListener(evt, () => {
                        this.lastActivity = Date.now();
                    });
                });

                if (!this.isLocked && await IsVaultSet()) {
                    await this.loadAccounts();
                    await this.loadTOTPs();
                    await this.sanitizeTOTPs();
                }

                // Batch Update Listener
                runtime.EventsOn("batch_update", (updates) => {
                    let changed = false;
                    Object.keys(updates).forEach(key => {
                        const [type, email] = key.split(':');
                        const acc = this.accounts.find(a => a.email === email);
                        if (acc) {
                            if (type === 'account_status') {
                                if (acc.status !== updates[key]) {
                                    acc.status = updates[key];
                                    changed = true;
                                }
                            }
                            if (type === 'unread_count') {
                                if (acc.unread_count !== updates[key]) {
                                    acc.unread_count = updates[key];
                                    changed = true;
                                }
                            }
                        }
                    });
                    if (changed) {
                        // Directly update sortedAccounts without full re-sort if possible
                        this.updateSortedAccounts(); 
                    }
                });

                runtime.EventsOn("NewEmailNotification", (data) => {
                    this.showToast(data.subject, `New Email: ${data.label}`, data.email);
                    if (this.settings.sound) {
                        this.notificationSound.currentTime = 0;
                        this.notificationSound.play().catch(e => console.error("Sound play error:", e));
                    }
                });

                // Load initial about info
                this.aboutInfo = await GetAboutInfo();
            } catch (e) {
                console.error("Initialization error:", e);
            }

                // Update check timer
            this.checkForUpdates();
            setInterval(() => this.checkForUpdates(), 3600000);

            setInterval(async () => {
                this.nextSyncSecs = await GetNextSync();
            }, 1000);

            runtime.EventsOn("sync_start", (email) => {
                this.updateAccountStatus(email, 'syncing');
            });

            runtime.EventsOn("sync_complete", async (email) => {
                const acc = this.accounts.find(a => a.email === email);
                if (acc) {
                    acc.status = 'connected';
                    await this.loadAccounts();
                    if (this.selectedAccount?.email === email) {
                        await this.loadCachedMessages(email);
                    }
                }
            });

            runtime.EventsOn("notification", (data) => {
                this.showToast(data.msg, data.title, data.email);
                if (this.settings.sound) {
                    this.notificationSound.currentTime = 0;
                    this.notificationSound.play().catch(e => console.error("Sound play error:", e));
                }
                this.loadAccounts();
            });

            runtime.EventsOn("sync_error", (email) => {
                const acc = this.accounts.find(a => a.email === email);
                if (acc) acc.status = 'error';
            });
        },

        async loadSettings() {
            this.settings = await GetSettings();
        },

        async submitUpdateSettings() {
            await UpdateSettings(this.settings);
            this.showSettings = false;
            this.showToast("Settings saved");
        },

        async loadAccounts() {
            try {
                const accs = await GetAccounts();
                const oldAccounts = [...this.accounts];
                this.accounts = (accs || []).map(a => {
                    const existing = oldAccounts.find(oa => oa.email === a.email);
                    return {
                        ...a,
                        status: existing ? existing.status : (this.settings.auto_login ? 'syncing' : 'disconnected')
                    };
                });
            } catch (e) {
                console.error("Load accounts error:", e);
                this.accounts = [];
            }
            this.updateSortedAccounts();
        },

        updateSortedAccounts() {
            this.sortedAccounts = [...this.accounts].sort((a, b) => (b.last_message_time || 0) - (a.last_message_time || 0));
        },

        async loadTOTPs() {
            try {
                this.stopAllTOTPTimers();
                this.totpList = (await GetTOTPList()) || [];
                
                // Initialize timers for each entry
                this.totpList.forEach(t => {
                    this.startTOTPTimer(t.account_name, t.secret);
                });
            } catch (e) {
                console.error("Load TOTPs error:", e);
                this.totpList = [];
            }
        },

        async sanitizeTOTPs() {
            console.log("[TOTP] Starting sanitization...");
            const brokenLabels = [];
            for (const t of this.totpList) {
                try {
                    await GenerateTOTP(t.secret);
                } catch (e) {
                    console.warn(`[TOTP Sanitization] Purging broken secret for ${t.account_name}:`, e);
                    brokenLabels.push(t.account_name);
                }
            }

            for (const label of brokenLabels) {
                this.stopTOTPTimer(label);
                await DeleteTOTP(label);
            }

            if (brokenLabels.length > 0) {
                await this.loadTOTPs();
                this.showToast(`Purged ${brokenLabels.length} corrupted TOTP entries`);
            }
        },

        startTOTPTimer(label, secret) {
            this.stopTOTPTimer(label); // Safety first
            
            const updateFunc = async () => {
                if (this.isLocked) return;
                try {
                    const res = await GenerateTOTP(secret);
                    const idx = this.totpList.findIndex(t => t.account_name === label);
                    if (idx !== -1) {
                        this.totpList[idx].code = res.code;
                        if (idx === 0) this.timeLeft = res.timeLeft;
                    }
                } catch (e) {
                    console.error(`[TOTP Zombie Fix] Stopping timer for ${label} due to error:`, e);
                    this.stopTOTPTimer(label);
                    const idx = this.totpList.findIndex(t => t.account_name === label);
                    if (idx !== -1) this.totpList[idx].code = "INVALID";
                }
            };

            updateFunc(); // Run once immediately
            this.totpTimers[label] = setInterval(updateFunc, 1000);
        },

        stopTOTPTimer(label) {
            if (this.totpTimers[label]) {
                clearInterval(this.totpTimers[label]);
                delete this.totpTimers[label];
            }
        },

        stopAllTOTPTimers() {
            Object.keys(this.totpTimers).forEach(label => this.stopTOTPTimer(label));
        },

        async updateTOTPs() {
            // Deprecated: Now using per-label timers for better lifecycle management
        },

        async selectAccount(acc) {
            // Lazy load full details
            this.selectedAccount = await GetAccountDetails(acc.email);
            this.messages = [];
            this.selectedMessage = null;
            this.messageBody = '';
            this.currentFolder = 'INBOX';
            await this.loadCachedMessages(acc.email);
            if (this.messages.length === 0) {
                await this.refreshInbox();
            }
        },

        async setFolder(folder) {
            this.currentFolder = folder;
            this.messages = [];
            this.selectedMessage = null;
            this.messageBody = '';
            if (folder === 'INBOX') {
                await this.loadCachedMessages(this.selectedAccount.email);
                if (this.messages.length === 0) await this.refreshInbox();
            } else if (folder === 'SPAM') {
                const spamName = await globalThis.go.app.App.DiscoverSpamFolder(this.selectedAccount.email);
                if (spamName) {
                    this.refreshing = true;
                    try {
                        this.messages = await globalThis.go.app.App.FetchEmails(this.selectedAccount.email, spamName, 50);
                    } catch (e) {
                        this.showToast("No Spam folder found or error");
                    } finally {
                        this.refreshing = false;
                    }
                }
            }
        },

        async loadCachedMessages(email) {
            const msgs = (await GetCachedMessages(email)) || [];
            if (msgs.length > 0) {
                const currentTopUid = this.messages.length > 0 ? this.messages[0].uid : 0;
                const newTopUid = msgs[0].uid;
                if (newTopUid !== currentTopUid || this.messages.length !== msgs.length) {
                    this.messages = msgs;
                }
            } else {
                this.messages = [];
            }
        },

        async refreshInbox(silent = false) {
            if (!this.selectedAccount) return;
            if (!silent) this.refreshing = true;
            try {
                let msgs;
                const folder = this.currentFolder || 'INBOX';
                if (folder === 'SPAM') {
                    const spamName = await globalThis.go.app.App.DiscoverSpamFolder(this.selectedAccount.email);
                    msgs = await globalThis.go.app.App.FetchEmails(this.selectedAccount.email, spamName, 50);
                } else {
                    msgs = await FetchEmails(this.selectedAccount.email, folder, 50);
                }
                this.messages = msgs || [];
                // Reset status locally for responsiveness
                const acc = this.accounts.find(a => a.email === this.selectedAccount.email);
                if (acc) acc.status = 'connected';
            } catch (e) {
                console.error("Refresh error:", e);
                this.showToast("Error refreshing folder");
            } finally {
                if (!silent) this.refreshing = false;
            }
        },

        async markAllAsRead() {
            if (!this.selectedAccount) return;
            try {
                const folder = this.currentFolder || 'INBOX';
                await MarkAllAsRead(this.selectedAccount.email, folder);
                this.messages.forEach(m => m.seen = true);
                if (folder === 'INBOX') {
                    const acc = this.accounts.find(a => a.email === this.selectedAccount.email);
                    if (acc) acc.unread_count = 0;
                }
                this.showToast("Folder marked as read");
            } catch (e) {
                console.error("Mark all error:", e);
                this.showToast("Failed to mark as read");
            }
        },

        async selectMessage(msg) {
            this.selectedMessage = msg;
            this.messageBody = '';
            this.bodyLoading = true;
            try {
                const folder = this.currentFolder || 'INBOX';
                const result = await FetchBody(this.selectedAccount.email, folder, msg.uid);
                if (result) {
                    this.messageBody = result[0];
                    this.selectedMessage.codes = result[1] || [];
                }
                if (!msg.seen) {
                    msg.seen = true;
                    const folder = this.currentFolder || 'INBOX';
                    await MarkAsRead(this.selectedAccount.email, folder, msg.uid);
                }
            } catch (e) {
                console.error("Error fetching body:", e);
                this.messageBody = "Error loading message body.";
            } finally {
                this.bodyLoading = false;
            }
        },

        async submitAddAccount() {
            if (!this.newAcc.email || !this.newAcc.password) {
                this.showToast("Email and password required");
                return;
            }
            try {
                const success = await AddAccount(
                    this.newAcc.email,
                    this.newAcc.password,
                    this.newAcc.host,
                    this.newAcc.port,
                    this.newAcc.label
                );
                if (success) {
                    await this.loadAccounts();
                    this.showAddAccount = false;
                    this.newAcc = { email: '', password: '', host: '', port: '', label: '' };
                    this.showToast("Account added");
                } else {
                    this.showToast("Account already exists or error");
                }
            } catch (e) {
                this.showToast("Error adding account");
            }
        },

        async deleteAccount(email) {
            if (await this.askConfirm("Delete Account", `Are you sure you want to delete account ${email}?`)) {
                await DeleteAccount(email);
                await this.loadAccounts();
                if (this.selectedAccount?.email === email) {
                    this.selectedAccount = null;
                    this.messages = [];
                }
                this.showToast("Account deleted");
            }
        },

        async submitAddTOTP() {
            if (!this.newTotp.secret || !this.newTotp.label) {
                this.showToast("Label and Secret required");
                return;
            }
            try {
                // Bridge expects AddTOTP(accountName, secret, issuer, account)
                await AddTOTP(
                    this.newTotp.label,
                    this.newTotp.secret.toUpperCase().replace(/\s/g, ''),
                    this.newTotp.issuer || '',
                    this.newTotp.account || ''
                );
                await this.loadTOTPs();
                this.showAddTOTP = false;
                this.newTotp = { label: '', secret: '', issuer: '', account: '' };
                this.showToast("TOTP added");
            } catch (e) {
                this.showToast("Error adding TOTP");
                console.error("AddTOTP Error:", e);
            }
        },

        async deleteTOTP(label) {
            // UNCONDITIONAL FORCE DELETION
            if (await this.askConfirm("Delete TOTP", `Are you sure? This will PERMANENTLY remove the secret for ${label}.`)) {
                try {
                    this.stopTOTPTimer(label);
                    // Force UI update even if backend fails
                    this.totpList = this.totpList.filter(t => t.account_name !== label);
                    await DeleteTOTP(label);
                    await this.loadTOTPs();
                    this.showToast("TOTP deleted");
                } catch (e) {
                    console.error("Force Delete Error (ignoring):", e);
                    this.showToast("TOTP removed from view");
                    await this.loadTOTPs();
                }
            }
        },

        async confirmRemove2FA(email) {
            const t = this.getLinkedTOTP(email);
            if (!t) return;
            if (await this.askConfirm("Remove 2FA", `Are you sure you want to remove 2FA protection from account ${email}?`)) {
                this.stopTOTPTimer(t.account_name);
                await DeleteTOTP(t.account_name);
                await this.loadTOTPs();
                this.showToast("2FA removed from account");
            }
        },

        async copyTOTP(totp) {
            if (!totp.code) return;
            const success = await CopyToClipboard(totp.code);
            if (success) this.showToast(`Copied ${totp.code}`);
        },

        async copyToClipboard(text) {
            const success = await CopyToClipboard(text);
            if (success) this.showToast(`Copied ${text}`);
        },

        showToast(msg, title = "System", email = "") {
            this.toast.msg = msg;
            this.toast.title = title;
            this.toast.email = email;
            this.toast.show = true;
            if (this.toastTimeout) clearTimeout(this.toastTimeout);
            this.toastTimeout = setTimeout(() => { this.toast.show = false; }, 5000);
        },

        handleNotificationClick(email) {
            if (!email) return;
            const acc = this.accounts.find(a => a.email === email);
            if (acc) {
                this.selectAccount(acc);
                this.sidebarMode = 'accounts';
                this.toast.show = false;
            }
        },

        get visibleAccounts() {
            const list = this.sortedAccounts;
            if (this.accountSearch) {
                const q = this.accountSearch.toLowerCase();
                const filtered = list.filter(a => a.email.toLowerCase().includes(q) || (a.label && a.label.toLowerCase().includes(q)));
                return this.sliceVisible(filtered);
            }
            return this.sliceVisible(list);
        },

        sliceVisible(list) {
            const startVisible = Math.floor(this.accountScrollTop / this.accountItemHeight);
            const endVisible = Math.ceil((this.accountScrollTop + this.accountViewportHeight) / this.accountItemHeight);
            
            const start = Math.max(0, startVisible - this.accountBuffer);
            const end = Math.min(list.length, endVisible + this.accountBuffer);
            
            return list.slice(start, end).map((acc, index) => ({
                ...acc,
                virtualTop: (start + index) * this.accountItemHeight
            }));
        },

        get accountsTotalHeight() {
            const list = this.accountSearch ? this.sortedAccounts.filter(a => {
                const q = this.accountSearch.toLowerCase();
                return a.email.toLowerCase().includes(q) || (a.label && a.label.toLowerCase().includes(q));
            }) : this.sortedAccounts;
            return list.length * this.accountItemHeight;
        },

        handleAccountScroll(e) {
            this.accountScrollTop = e.target.scrollTop;
            this.accountViewportHeight = e.target.offsetHeight;
            
            // Notify backend about visible accounts (debounced)
            clearTimeout(this.scrollFinishTimeout);
            this.scrollFinishTimeout = setTimeout(() => {
                const visibleEmails = this.visibleAccounts.map(a => a.email);
                SetVisibleAccounts(visibleEmails);
            }, 500);
        },

        async askConfirm(title, message) {
            this.dialog = { show: true, title, message, type: 'confirm', resolve: null };
            return new Promise(resolve => {
                this.dialog.resolve = resolve;
            });
        },

        async showAlert(title, message) {
            this.dialog = { show: true, title, message, type: 'alert', resolve: null };
            return new Promise(resolve => {
                this.dialog.resolve = resolve;
            });
        },

        handleDialog(value) {
            const resolve = this.dialog.resolve;
            this.dialog.show = false;
            if (resolve) resolve(value);
        },

        getLinkedTOTP(email) {
            return this.totpList.find(t => t.account === email);
        },

        async copyLinkedTOTP(email) {
            const t = this.getLinkedTOTP(email);
            if (t && t.code) {
                const success = await CopyToClipboard(t.code);
                if (success) this.showToast(`Copied TOTP: ${t.code}`);
            }
        },

        exportProfile() {
            this.exportPass = '';
            this.exportPath = 'profile_backup.vaultix';
            this.showExportModal = true;
        },

        async pickExportPath() {
            const path = await SelectSavePath();
            if (path) this.exportPath = path;
        },

        async submitExportProfile() {
            if (!this.exportPass || !this.exportPath) return;
            try {
                await ExportProfile(this.exportPass, this.exportPath);
                this.showToast("Profile exported to " + this.exportPath);
                this.showExportModal = false;
            } catch (e) {
                this.showToast("Export failed: " + e);
            }
        },

        importProfile() {
            this.importPass = '';
            this.importPath = '';
            this.showImportModal = true;
        },

        async pickImportPath() {
            const path = await SelectOpenPath();
            if (path) this.importPath = path;
        },

        async submitImportProfile() {
            if (!this.importPass || !this.importPath) return;
            try {
                await ImportProfile(this.importPass, this.importPath);
                this.showToast("Profile imported successfully. Please restart.");
                this.showImportModal = false;
                await this.loadAccounts();
                await this.loadSettings();
                this.isUnsecuredImport = await IsUnsecuredImport();
                this.showPasswordSetup = await NeedsSetup();
            } catch (e) {
                this.showToast("Import failed: " + e);
            }
        },

        async openChangePassword() {
            this.passwordSet = await IsPasswordSet();
            this.changePassOld = '';
            this.changePassNew = '';
            this.changePassConfirm = ''; 
            this.showChangePassword = true;
            this.showSettings = false;
        },

        async submitSmartPassword() {
            // Logic for "Smart Password Management" section
            const passwordExists = await IsPasswordSet();
            
            if (!passwordExists) {
                // Set Password Flow (New + Confirm)
                if (!this.changePassNew || !this.changePassConfirm) {
                    this.showToast("Please fill all fields", "Security");
                    return;
                }
                if (this.changePassNew !== this.changePassConfirm) {
                    this.showToast("Passwords do not match", "Security");
                    return;
                }
                await SetAppPassword(this.changePassNew);
                this.showToast("Master password set successfully");
            } else {
                // Change Password Flow (Old + New + Confirm)
                if (!this.changePassOld || !this.changePassNew || !this.changePassConfirm) {
                    this.showToast("Please fill all fields", "Security");
                    return;
                }
                if (this.changePassNew !== this.changePassConfirm) {
                    this.showToast("New passwords do not match", "Security");
                    return;
                }
                try {
                    const { ChangeAppPassword } = globalThis.go.app.App;
                    await ChangeAppPassword(this.changePassOld, this.changePassNew);
                    this.showToast("Vault password updated");
                } catch (e) {
                    this.showToast("Error: " + e, "Security");
                    return;
                }
            }
            this.showChangePassword = false;
            this.changePassNew = '';
            this.changePassOld = '';
            this.changePassConfirm = '';
        },

        async submitBulkImport() {
            if (!this.bulkImapServer) {
                this.showToast("Please enter an IMAP server");
                return;
            }
            try {
                const { SelectBulkImportPath, BulkAddAccounts } = globalThis.go.app.App;
                const path = await SelectBulkImportPath();
                if (!path) return;

                const count = await BulkAddAccounts(path, this.bulkImapServer);
                if (count > 0) {
                    this.showToast(`Successfully imported ${count} accounts`);
                    await this.loadAccounts();
                    this.showBulkAdd = false;
                } else {
                    this.showToast("No accounts were imported. Check file format.");
                }
            } catch (e) {
                this.showToast("Bulk import failed: " + e);
            }
        },

        get timerClass() {
            if (this.timeLeft > 15) return 'text-green-400';
            if (this.timeLeft > 7) return 'text-yellow-400';
            return 'text-red-400';
        },

        get timerBgClass() {
            if (this.timeLeft > 15) return 'bg-green-500';
            if (this.timeLeft > 7) return 'bg-yellow-500';
            return 'bg-red-500';
        },

        async submitSetupPassword() {
            if (!this.setupPass) return;
            await SetAppPassword(this.setupPass);
            this.showPasswordSetup = false;
            this.setupPass = '';
            this.showToast("Master password protection enabled");
            await this.loadAccounts();
            await this.loadTOTPs();
        },


        async skipSetupPassword() {
            if (await this.askConfirm("Skip Protection", "Are you sure? Your data will be stored without app-level password protection.")) {
                await SkipAppPasswordSetup();
                this.showPasswordSetup = false;
                this.showToast("Password protection skipped");
                await this.loadAccounts();
                await this.loadTOTPs();
            }
        },

        async submitUnlock() {
            if (!this.lockPass) return;
            const success = await UnlockApp(this.lockPass);
            if (success) {
                this.isLocked = false;
                this.lockPass = '';
                this.lastActivity = Date.now();
                await this.loadAccounts();
                await this.loadTOTPs();
                // Ensure engine starts if enabled (handled in backend but good to know)
                this.showToast("Vault unlocked");
            } else {
                this.showToast("Invalid password", "Security");
                this.lockPass = '';
            }
        },

        async openAbout() {
            this.aboutInfo = await GetAboutInfo();
            this.showAbout = true;
        },

        async lockAppManual() {
            if (await IsPasswordSet()) {
                await LockVault();
                this.isLocked = true;
                this.accounts = [];
                this.sortedAccounts = [];
                this.selectedAccount = null;
                this.messages = [];
                this.showToast("Vault locked and keys cleared", "Security");
            } else {
                this.showToast("Setup master password in settings first", "Security");
                this.showSettings = true;
            }
        },

        async checkForUpdates() {
            try {
                const info = await CheckForUpdates();
                if (info && info.has_update) {
                    this.updateInfo = info;
                }
            } catch (e) {
                console.error("Failed to check for updates:", e);
            }
        },

        updateAccountStatus(email, status) {
            const acc = this.accounts.find(a => a.email === email);
            if (acc) acc.status = status;
        },
    }));

    // Global Error Handler for better debugging
    globalThis.addEventListener('error', (event) => {
        console.error("Global JS Error:", event.error);
        // Don't show toast here as it might not be initialized, 
        // but it will be visible in DevTools
    });

    globalThis.addEventListener('unhandledrejection', (event) => {
        console.error("Unhandled Promise Rejection:", event.reason);
    });
});

Alpine.start();
