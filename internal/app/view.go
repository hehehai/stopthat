package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/hehehai/stopthat/internal/firewall"
	"github.com/hehehai/stopthat/internal/presets"
)

// View renders the entire StopThat desktop UI for MyGo.
func (s *State) View(c *ui.Context) {
	t := c.Theme()

	root := ui.Row(c).Fill().Background(t.Background).Children(func() {
		s.renderSidebar(c, t)
		ui.Divider(c)
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			s.renderBanners(c, t)
			switch s.Section {
			case SectionAdobe:
				s.renderPresetView(c, t, "adobe")
			case SectionCustom:
				s.renderCustomView(c, t)
			case SectionRules:
				s.renderRulesView(c, t)
			case SectionDiagnostics:
				s.renderDiagnosticsView(c, t)
			default:
				s.renderPresetView(c, t, "adobe")
			}
		})
	})
	if dropped := root.DroppedFiles(); len(dropped) > 0 {
		s.HandleDroppedFiles(dropped)
	}
}

func (s *State) renderSidebar(c *ui.Context, t *ui.Theme) {
	totalAdobe, _, _, blockedAdobe := s.PresetStats("adobe")
	ruleCount := len(s.rules)
	customCount := len(s.customTargets)

	ui.Column(c).Width(224).FillHeight().Background(t.Surface).Padding(16, 12).Gap(12).Children(func() {
		// Brand header
		ui.Column(c).Padding(4, 8).Gap(2).Children(func() {
			ui.Text(c, "StopThat").FontSize(18).Bold()
			ui.Text(c, "应用联网权限管理").FontSize(12).TextColor(t.TextMuted)
		})

		ui.Divider(c)

		// Navigation
		ui.Sidebar(c, &s.Section, func() {
			ui.Text(c, "预设应用套件").FontSize(11).TextColor(t.TextMuted).Padding(6, 8, 2, 8)
			adobeLabel := "Adobe 套件"
			if totalAdobe > 0 {
				adobeLabel = fmt.Sprintf("Adobe 套件 (%d/%d)", blockedAdobe, totalAdobe)
			}
			ui.SidebarItem(c, SectionAdobe, nil, adobeLabel)

			ui.Text(c, "通用管理").FontSize(11).TextColor(t.TextMuted).Padding(12, 8, 2, 8)
			customLabel := "自定义应用"
			if customCount > 0 {
				customLabel = fmt.Sprintf("自定义应用 (%d)", customCount)
			}
			ui.SidebarItem(c, SectionCustom, nil, customLabel)

			rulesLabel := "已生效规则"
			if ruleCount > 0 {
				rulesLabel = fmt.Sprintf("已生效规则 (%d)", ruleCount)
			}
			ui.SidebarItem(c, SectionRules, nil, rulesLabel)
			ui.SidebarItem(c, SectionDiagnostics, nil, "网络体检")
		}).Grow(1)

		ui.Divider(c)

		// Engine footer card
		ui.Column(c).Padding(10).Radius(8).Background(t.Background).Border(1, t.Border).Gap(4).Children(func() {
			ui.Text(c, s.caps.EngineName).FontSize(12).Bold()
			statusText := "权限正常"
			statusColor := t.Success
			if !s.caps.Elevated && s.caps.Platform == "windows" {
				statusText = "需要管理员权限"
				statusColor = t.Danger
			} else if s.caps.Platform == "darwin" {
				statusText = "操作时按需授权"
				statusColor = t.TextMuted
			}
			ui.Text(c, statusText).FontSize(11).TextColor(statusColor)
		})
	})
}

func (s *State) renderBanners(c *ui.Context, t *ui.Theme) {
	if s.caps.NeedsElevationHint != "" {
		ui.Row(c).FillWidth().Padding(10, 20).Background(t.Danger).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, s.caps.NeedsElevationHint).FontSize(12).TextColor(t.AccentText).Grow(1)
			if firewall.CanRelaunchElevated() {
				if ui.Button(c.Key("relaunch-admin"), "以管理员身份重启").Clicked() {
					if err := firewall.RelaunchElevated(); err != nil {
						s.StatusMsg = err.Error()
						s.StatusErr = true
					}
				}
			}
		})
	}
	if s.Busy {
		ui.Row(c).FillWidth().Padding(8, 20).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Spinner(c)
			msg := s.BusyMsg
			if msg == "" {
				msg = "正在处理防火墙规则，请稍候..."
			}
			ui.Text(c, msg).FontSize(12).TextColor(t.Text)
		})
	}
	if s.StatusMsg != "" {
		bg := t.Surface
		fg := t.Text
		if s.StatusErr {
			bg = t.Danger
			fg = t.AccentText
		}
		ui.Row(c).FillWidth().Padding(8, 20).Background(bg).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, s.StatusMsg).FontSize(12).TextColor(fg).Grow(1)
			if ui.Button(c.Key("dismiss-status"), "关闭").Clicked() {
				s.StatusMsg = ""
				s.StatusErr = false
			}
		})
	}
}

func (s *State) renderPresetView(c *ui.Context, t *ui.Theme, presetID string) {
	preset, ok := s.PresetByID(presetID)
	if !ok {
		return
	}
	allTargets := s.presetTargets[presetID]
	total, mainCount, bgCount, blockedCount := s.PresetStats(presetID)
	filtered := s.FilteredTargets(allTargets)

	ui.Column(c).Fill().Padding(20).Gap(14).Children(func() {
		// Header summary card
		ui.Column(c).FillWidth().Padding(16).Radius(10).Background(t.Surface).Border(1, t.Border).Gap(12).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					ui.Text(c, preset.Name).FontSize(18).Bold()
					ui.Text(c, preset.Description).FontSize(12).TextColor(t.TextMuted)
				})
				ui.Row(c).Gap(8).Children(func() {
					if ui.PrimaryButton(c.Key("block-all"), "一键全部阻止").Disabled(total == 0 || s.Busy).Clicked() {
						s.RunAsync("正在批量写入防火墙阻止规则...", func() { s.BlockPresetAll(presetID) })
					}
					if ui.Button(c.Key("unblock-all"), "一键全部恢复").Disabled(blockedCount == 0 || s.Busy).Clicked() {
						s.RunAsync("正在批量恢复联网权限...", func() { s.UnblockPresetAll(presetID) })
					}
				})
			})

			ui.Divider(c)

			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Textf(c, "已扫描 %d 个程序（主程序 %d · 后台组件 %d · 已阻止 %d）",
					total, mainCount, bgCount, blockedCount).FontSize(12).TextColor(t.TextMuted).Grow(1)
				if ui.Button(c.Key("copy-preset-paths"), "复制路径清单").Disabled(total == 0).Clicked() {
					c.WriteClipboard(FormatPathsText(allTargets))
					s.StatusMsg = fmt.Sprintf("已复制 %d 条程序路径到剪贴板（可用于火绒或代理规则）", total)
					s.StatusErr = false
				}
				if ui.Button(c.Key("add-preset-dir"), "添加安装目录").Disabled(s.Busy).Clicked() {
					s.PickAndAddPresetFolder(presetID)
				}
				if ui.Button(c.Key("rescan-preset"), "重新扫描").Disabled(s.Busy).Clicked() {
					s.RunAsync("正在重新扫描安装目录与防火墙规则...", func() {
						s.RefreshAll()
						s.mu.Lock()
						s.setStatusLocked(fmt.Sprintf("重新扫描完成，共发现 %d 个 %s 程序", len(s.presetTargets[presetID]), preset.ShortName), false)
						s.mu.Unlock()
					})
				}
			})

			// Policy checkboxes
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(18).Children(func() {
				if ui.Checkbox(c.Key("opt-out"), &s.cfg.BlockOutbound, "阻止出站连接（推荐）").Disabled(!s.caps.SupportsOutbound).Changed() {
					s.SavePreferences()
				}
				if ui.Checkbox(c.Key("opt-in"), &s.cfg.BlockInbound, "阻止入站连接").Changed() {
					s.SavePreferences()
				}
				if ui.Checkbox(c.Key("opt-bg"), &s.cfg.IncludeBackground, "一键阻止时包含后台组件").Changed() {
					s.SavePreferences()
				}
			})

			if s.caps.OutboundNote != "" {
				ui.Text(c, s.caps.OutboundNote).FontSize(11).TextColor(t.Warning)
			}
		})

		// Filter bar
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
			ui.SearchField(c.Key("search-targets"), &s.SearchQuery).Width(240)
			ui.Segmented(c.Key("filter-kind"), &s.KindFilter, "全部", "主程序", "后台组件", "已阻止", "未阻止")
		})

		// Target list
		if len(allTargets) == 0 {
			s.renderEmptyPreset(c, t, presetID)
		} else if len(filtered) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(8).Children(func() {
				ui.Text(c, "没有符合当前筛选条件的程序").FontSize(14).TextColor(t.TextMuted)
			})
		} else {
			ui.Scroll(c).Grow(1).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Gap(8).Padding(0, 4, 8, 0).Children(func() {
					for _, target := range filtered {
						s.renderTargetRow(c, t, target)
					}
				})
			})
		}
	})
}

func (s *State) renderEmptyPreset(c *ui.Context, t *ui.Theme, presetID string) {
	ui.Column(c).Grow(1).FillWidth().Center().Padding(24).Radius(10).Background(t.Surface).Border(1, t.Border).Gap(10).Children(func() {
		ui.Text(c, "在默认安装目录中未检测到 Adobe 程序").FontSize(15).Bold()
		ui.Text(c, "如果你将软件安装在自定义盘符（例如 D:\\Adobe），可点击「添加安装目录」或在下方粘贴目录路径。").FontSize(12).TextColor(t.TextMuted)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c.Key("preset-path-input"), &s.PathInput).Width(340)
			if ui.PrimaryButton(c.Key("preset-path-add"), "添加并扫描").Clicked() {
				s.AddExtraPresetRoot(presetID, s.PathInput)
				s.PathInput = ""
			}
		})
	})
}

func (s *State) renderTargetRow(c *ui.Context, t *ui.Theme, target presets.Target) {
	blocked := s.IsBlocked(target.Path)
	on := blocked

	ui.Row(c.Key("row-"+target.ID)).FillWidth().Padding(12, 14).Radius(8).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).Gap(12).Children(func() {
		ui.Column(c).Grow(1).Gap(3).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, target.Name).FontSize(13).Bold()
				ui.Badge(c, target.Kind.Label())
				if target.Note != "" {
					ui.Text(c, target.Note).FontSize(11).TextColor(t.TextMuted)
				}
			})
			ui.Text(c, target.Path).FontSize(11).TextColor(t.TextMuted)
		})

		statusLabel := "允许联网"
		statusColor := t.TextMuted
		if blocked {
			statusLabel = "已阻止联网"
			statusColor = t.Danger
		}
		ui.Text(c, statusLabel).FontSize(12).TextColor(statusColor)

		if ui.Switch(c.Key("sw-"+target.ID), &on).Disabled(s.Busy).Changed() {
			wantBlock := on
			tgt := target
			s.RunAsync("正在更新防火墙规则...", func() { s.ToggleTarget(tgt, wantBlock) })
		}
	})
}

func (s *State) renderCustomView(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Padding(20).Gap(14).Children(func() {
		ui.Column(c).FillWidth().Padding(16).Radius(10).Background(t.Surface).Border(1, t.Border).Gap(10).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "自定义应用与目录").FontSize(18).Bold()
					ui.Text(c, "添加任意可执行文件（.exe / .app）或安装目录，StopThat 会自动扫描其中的可执行程序供你按需开关联网权限。").FontSize(12).TextColor(t.TextMuted)
				})
				ui.Row(c).Gap(8).Children(func() {
					if ui.Button(c.Key("pick-file"), "选择程序").Disabled(s.Busy).Clicked() {
						s.PickAndAddCustomPath(false)
					}
					if ui.Button(c.Key("pick-dir"), "选择目录").Disabled(s.Busy).Clicked() {
						s.PickAndAddCustomPath(true)
					}
					if ui.Button(c.Key("copy-custom-paths"), "复制路径").Disabled(len(s.customTargets) == 0).Clicked() {
						c.WriteClipboard(FormatPathsText(s.customTargets))
						s.StatusMsg = fmt.Sprintf("已复制 %d 条自定义程序路径到剪贴板", len(s.customTargets))
						s.StatusErr = false
					}
					if ui.PrimaryButton(c.Key("block-custom-all"), "全部阻止").Disabled(len(s.customTargets) == 0 || s.Busy).Clicked() {
						s.RunAsync("正在批量阻止自定义程序联网...", func() { s.BlockCustomAll() })
					}
				})
			})

			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.TextInput(c.Key("custom-path-input"), &s.PathInput).Grow(1)
				if ui.Button(c.Key("add-custom-path"), "添加路径").Clicked() {
					s.AddCustomPath(s.PathInput)
					s.PathInput = ""
				}
			})

			if len(s.cfg.CustomPaths) > 0 {
				ui.Column(c).FillWidth().Gap(4).Children(func() {
					ui.Text(c, "已添加的路径：").FontSize(11).TextColor(t.TextMuted)
					for _, cp := range s.cfg.CustomPaths {
						pathCopy := cp
						ui.Row(c.Key("cp-"+pathCopy)).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
							ui.Text(c, pathCopy).FontSize(12).Grow(1)
							if ui.Button(c.Key("rm-cp-"+pathCopy), "移除").Clicked() {
								s.RemoveCustomPath(pathCopy)
							}
						})
					}
				})
			}
		})

		if len(s.customTargets) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(6).Children(func() {
				ui.Text(c, "暂未添加自定义应用").FontSize(14).TextColor(t.TextMuted)
				ui.Text(c, "支持直接将桌面快捷方式（.lnk）、程序（.exe / .app）或整个文件夹拖入窗口。").FontSize(12).TextColor(t.TextMuted)
			})
		} else {
			ui.Scroll(c).Grow(1).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Gap(8).Padding(0, 4, 8, 0).Children(func() {
					for _, target := range s.customTargets {
						s.renderTargetRow(c, t, target)
					}
				})
			})
		}
	})
}

func (s *State) renderRulesView(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Padding(20).Gap(14).Children(func() {
		ui.Row(c).FillWidth().Padding(16).Radius(10).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Textf(c, "已生效的 StopThat 规则（%d 条）", len(s.rules)).FontSize(18).Bold()
				ui.Text(c, "所有规则均带有 StopThat 专属前缀，不会影响你或系统原有的其他防火墙规则，支持随时一键还原。").FontSize(12).TextColor(t.TextMuted)
			})
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c.Key("refresh-rules"), "刷新").Disabled(s.Busy).Clicked() {
					s.RunAsync("正在刷新防火墙规则列表...", func() {
						s.RefreshAll()
						s.mu.Lock()
						s.setStatusLocked(fmt.Sprintf("已刷新，当前共 %d 条生效规则", len(s.rules)), false)
						s.mu.Unlock()
					})
				}
				if ui.Button(c.Key("remove-all-rules"), "一键全部还原").Disabled(len(s.rules) == 0 || s.Busy).Clicked() {
					s.RunAsync("正在一键还原全部 StopThat 规则...", func() { s.RemoveAllManagedRules() })
				}
			})
		})

		if len(s.rules) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(6).Children(func() {
				ui.Text(c, "当前没有由 StopThat 创建的阻止规则").FontSize(14).TextColor(t.TextMuted)
			})
		} else {
			ui.Scroll(c).Grow(1).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Gap(8).Padding(0, 4, 8, 0).Children(func() {
					for _, r := range s.rules {
						rule := r
						dirText := "出站"
						if rule.Direction == firewall.DirIn {
							dirText = "入站"
						}
						ui.Row(c.Key("rule-"+rule.Name)).FillWidth().Padding(12, 14).Radius(8).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).Gap(12).Children(func() {
							ui.Column(c).Grow(1).Gap(3).Children(func() {
								ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
									ui.Text(c, rule.DisplayName).FontSize(13).Bold()
									ui.Badge(c, dirText)
									ui.Text(c, rule.Name).FontSize(11).TextColor(t.TextMuted)
								})
								ui.Text(c, rule.ProgramPath).FontSize(11).TextColor(t.TextMuted)
							})
							if ui.Button(c.Key("rm-rule-"+rule.Name), "恢复联网").Disabled(s.Busy).Clicked() {
								p := rule.ProgramPath
								s.RunAsync("正在恢复程序联网...", func() { s.UnblockPath(p) })
							}
						})
					}
				})
			})
		}
	})
}

func (s *State) renderDiagnosticsView(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Padding(20).Gap(14).Children(func() {
		ui.Row(c).FillWidth().Padding(16).Radius(10).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "网络与防火墙环境体检").FontSize(18).Bold()
				ui.Text(c, "排查「已加规则但仍未生效」的常见原因：防火墙未启用、第三方安全软件接管、系统代理/TUN 绕过等。").FontSize(12).TextColor(t.TextMuted)
			})
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c.Key("open-fw-settings"), "打开系统防火墙").Clicked() {
					s.OpenSystemFirewallSettings()
				}
				if ui.Button(c.Key("flush-dns"), "刷新 DNS 缓存").Disabled(s.Busy).Clicked() {
					s.RunAsync("正在刷新 DNS 缓存...", func() { s.FlushDNS() })
				}
				if ui.PrimaryButton(c.Key("recheck-diag"), "重新体检").Disabled(s.Busy).Clicked() {
					s.RunAsync("正在检测网络与防火墙环境...", func() { s.RefreshDiagnostics() })
				}
			})
		})

		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(10).Padding(0, 4, 8, 0).Children(func() {
				for i, d := range s.diagnostics {
					levelColor := t.TextMuted
					levelLabel := "说明"
					switch d.Level {
					case firewall.DiagOK:
						levelColor = t.Success
						levelLabel = "正常"
					case firewall.DiagWarn:
						levelColor = t.Warning
						levelLabel = "注意"
					case firewall.DiagError:
						levelColor = t.Danger
						levelLabel = "异常"
					}
					ui.Column(c.Key(fmt.Sprintf("diag-%d", i))).FillWidth().Padding(14).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(6).Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
							ui.Text(c, d.Title).FontSize(14).Bold()
							ui.Badge(c, levelLabel)
							ui.Spacer(c)
							ui.Text(c, strings.TrimSpace(d.Status)).FontSize(12).TextColor(levelColor).Bold()
						})
						ui.Text(c, d.Detail).FontSize(12).TextColor(t.TextMuted)
					})
				}
			})
		})
	})
}
