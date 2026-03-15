import Alpine from 'alpinejs';
import './style.css';
import {
    GetAccounts,
    AddAccount,
    DeleteAccount,
    GetTOTPList,
    AddTOTP,
    DeleteTOTP,
    GenerateTOTP,
    FetchInbox,
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
    LockApp,
    CopyToClipboard,
    GetAboutInfo,
    CheckForUpdates
} from '../wailsjs/go/app/App';
import * as runtime from '../wailsjs/runtime';

globalThis.Alpine = Alpine;

console.log("VaultixIMQ main.js loading...");

document.addEventListener('alpine:init', () => {
    console.log("Alpine initialized, registering vaultixApp...");
    Alpine.data('vaultixApp', () => ({
        accounts: [],
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
        accountSearch: '',

        isLocked: false,
        showPasswordSetup: false,
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
            sync_interval: 10,
            auto_login: true,
            notifications: true,
            sound: true,
            auto_lock_interval: 5
        },
        nextSyncSecs: 0,
        bulkImapServer: '',
        notificationSound: new Audio('/notifications.wav'),

        openURL(url) {
            if (url) runtime.BrowserOpenURL(url);
        },

        async init() {
            try {
                await this.loadSettings();
                this.isLocked = await IsLocked();
                this.showPasswordSetup = await NeedsSetup();

                if (!this.isLocked) {
                    await this.loadAccounts();
                    await this.loadTOTPs();
                }

                // Load initial about info
                this.aboutInfo = await GetAboutInfo();
            } catch (e) {
                console.error("Initialization error:", e);
            }

            // Inactivity Tracker
            // Inactivity Tracker
            const updateActivity = () => { this.lastActivity = Date.now(); };
            globalThis.addEventListener('mousemove', updateActivity);
            globalThis.addEventListener('keydown', updateActivity);
            globalThis.addEventListener('click', updateActivity);

            setInterval(async () => {
                if (!this.isLocked && this.settings.auto_lock_interval > 0) {
                    const elapsed = (Date.now() - this.lastActivity) / 1000 / 60;
                    if (elapsed >= this.settings.auto_lock_interval && await IsPasswordSet()) {
                        await LockApp();
                        this.isLocked = true;
                        this.showToast("Application locked due to inactivity");
                    }
                }
            }, 10000);

            // Update check timer
            this.checkForUpdates();
            setInterval(() => this.checkForUpdates(), 3600000);

            setInterval(async () => {
                if (!this.isLocked) {
                    await this.updateTOTPs();
                }
            }, 1000);

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
        },

        async loadTOTPs() {
            try {
                this.totpList = (await GetTOTPList()) || [];
                await this.updateTOTPs();
            } catch (e) {
                console.error("Load TOTPs error:", e);
                this.totpList = [];
            }
        },

        async updateTOTPs() {
            try {
                const results = await Promise.all(this.totpList.map(t => GenerateTOTP(t.secret)));
                results.forEach((res, i) => {
                    this.totpList[i].code = res.code;
                    if (i === 0) this.timeLeft = res.timeLeft;
                });
            } catch (e) {
                console.error("TOTP update error:", e);
            }
        },

        async selectAccount(acc) {
            this.selectedAccount = acc;
            this.messages = [];
            this.selectedMessage = null;
            this.messageBody = '';
            await this.loadCachedMessages(acc.email);
            if (this.messages.length === 0) {
                await this.refreshInbox();
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
                const msgs = await FetchInbox(this.selectedAccount.email, 50);
                this.messages = msgs || [];
                this.selectedAccount.status = 'connected';
            } catch (e) {
                console.error("Refresh inbox error:", e);
                this.showToast("Error: Connection failed");
                this.selectedAccount.status = 'error';
            } finally {
                if (!silent) this.refreshing = false;
            }
        },

        async markAllAsRead() {
            if (!this.selectedAccount) return;
            try {
                await MarkAllAsRead(this.selectedAccount.email);
                this.messages.forEach(m => m.seen = true);
                this.selectedAccount.unread_count = 0;
                const acc = this.accounts.find(a => a.email === this.selectedAccount.email);
                if (acc) acc.unread_count = 0;
                this.showToast("All messages marked as read");
            } catch (e) {
                console.error("Mark all as read error:", e);
                this.showToast("Failed to mark all as read");
            }
        },

        async selectMessage(msg) {
            this.selectedMessage = msg;
            this.messageBody = '';
            this.bodyLoading = true;
            try {
                const result = await FetchBody(this.selectedAccount.email, msg.uid);
                if (result) {
                    this.messageBody = result[0];
                    this.selectedMessage.codes = result[1] || [];
                }
                if (!msg.seen) {
                    msg.seen = true;
                    await MarkAsRead(this.selectedAccount.email, msg.uid);
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
                await AddTOTP(
                    this.newTotp.label,
                    this.newTotp.secret.toUpperCase().replace(/\s/g, ''),
                    this.newTotp.issuer,
                    this.newTotp.account
                );
                await this.loadTOTPs();
                this.showAddTOTP = false;
                this.newTotp = { label: '', secret: '', issuer: '', account: '' };
                this.showToast("TOTP added");
            } catch (e) {
                this.showToast("Error adding TOTP");
            }
        },

        async deleteTOTP(label) {
            if (await this.askConfirm("Delete TOTP", `Are you sure you want to delete the TOTP secret for ${label}?`)) {
                await DeleteTOTP(label);
                await this.loadTOTPs();
                this.showToast("TOTP deleted");
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

        filteredAccounts() {
            let list = [...this.accounts];
            if (this.accountSearch) {
                const q = this.accountSearch.toLowerCase();
                list = list.filter(a => a.email.toLowerCase().includes(q) || (a.label && a.label.toLowerCase().includes(q)));
            }
            // Sort by last_message_time (descending)
            return list.sort((a, b) => (b.last_message_time || 0) - (a.last_message_time || 0));
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
            } catch (e) {
                this.showToast("Import failed: " + e);
            }
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
                await LockApp();
                this.isLocked = true;
                this.showToast("Application locked manually");
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
});

Alpine.start();
