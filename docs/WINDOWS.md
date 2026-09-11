# Windows 使用与排障

## 1. 下载正确的产物（最常见问题）

Win10 打不开 App，最常见的原因是**下载了 macOS 产物**：

| 文件 | 平台 | 说明 |
|---|---|---|
| `Etcura.app` / `etcura-darwin-arm64.zip` | **仅 macOS** | Mach-O 可执行文件，Windows 无法运行，双击无反应属于正常现象 |
| `etcura.exe` / `etcura-windows-amd64.zip` | **Windows 10/11 x64** | 解压后双击 `etcura.exe` 运行 |

判断方法：在 Windows 上右键文件 → 属性，若类型显示为“应用程序”且扩展名是 `.exe` 才可用；
`.app` 是 macOS 的应用目录（在 Windows 上看起来像个普通文件夹），无法运行。

## 2. 运行前需要 Microsoft Edge WebView2 运行时

Etcura 的界面由系统 WebView2 渲染，Windows 10 **不一定自带**该运行时。

本项目发布的 `etcura.exe` 使用 **embedded WebView2 bootstrapper** 构建，
如果目标机器缺少运行时会自动尝试安装。如果你的网络环境无法访问微软下载源，
或仍然无法启动，请手动安装：

- 官方下载（Evergreen Bootstrapper）：
  https://developer.microsoft.com/microsoft-edge/webview2/
- 或使用 winget：

```powershell
winget install Microsoft.EdgeWebView2Runtime
```

安装后重新双击 `etcura.exe` 即可。

**如何确认是不是这个原因**：在 PowerShell 中运行（可看到报错输出）

```powershell
cd <解压目录>
.\etcura.exe
```

若提示 WebView2 相关错误，按上面安装运行时即可。

## 3. SmartScreen / 杀毒软件拦截

发布的 exe 未做代码签名（Windows 代码签名需要付费证书），首次运行可能被拦截：

- **SmartScreen 蓝色窗口**：“Windows 已保护你的电脑” → 点击**更多信息** → **仍要运行**
- **杀毒软件误报**：Wails 打包的 exe 偶发被误判，添加信任/白名单即可

若公司电脑有安全策略限制未签名程序，需要联系 IT 或自行签名后分发。

## 4. 其他检查项

- **架构**：发布的默认产物是 `amd64`（x86-64），适用于绝大多数 Win10/11 机器。
  极少数 ARM 版 Windows 请使用 `arm64` 产物。
- **防火墙**：连接远端 etcd 时若超时，检查出站规则是否放通对应端口（如 2379/32379）。
- **凭据存储**：Windows 上密码保存在 **Windows 凭据管理器**（凭据管理器 → Windows 凭据，
  目标名以 `etcura` 开头），与 macOS 的 Keychain 行为一致；配置文件在
  `%AppData%\etcura\connections.json`（不含密码）。

## 5. 从源码自行构建

在 macOS/Linux 上交叉编译（需要 mingw-w64）：

```bash
brew install mingw-w64            # macOS
./scripts/build-windows.sh        # 产出 build/bin/etcura.exe + zip
```

在 Windows 本机构建：安装 Go、Node、Wails CLI 与 WebView2 运行时后，直接执行

```powershell
wails build -platform windows/amd64
```
