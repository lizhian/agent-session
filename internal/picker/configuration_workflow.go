package picker

import (
	"github.com/lizhian/agent-session/internal/provider"
	"github.com/lizhian/agent-session/internal/render"
)

type configurationWorkflow struct {
	provider provider.Provider

	actions  []provider.ConfigAction
	items    []provider.ConfigItem
	subitems []provider.ConfigItem

	activeAction   *provider.ConfigAction
	activeItem     *provider.ConfigItem
	activeSubitems *provider.SubitemConfigAction
	status         string

	selectedIndex     int
	itemSelectedIndex int
}

func newConfigurationWorkflow(p provider.Provider) configurationWorkflow {
	return configurationWorkflow{
		provider: p,
		actions:  p.ConfigurationActions(),
	}
}

func (w *configurationWorkflow) openConfigurations() {
	w.selectedIndex = 0
	w.status = ""
}

func (w *configurationWorkflow) resetActive() {
	w.activeAction = nil
	w.activeItem = nil
	w.activeSubitems = nil
	w.items = nil
	w.subitems = nil
}

func (w *configurationWorkflow) cancelItems() {
	w.resetActive()
}

func (w *configurationWorkflow) cancelSubitems() View {
	w.activeItem = nil
	w.activeSubitems = nil
	w.subitems = nil
	if w.activeAction != nil && w.activeAction.DirectMultiSelect != nil {
		w.activeAction = nil
		return ViewConfigurations
	}
	return ViewConfigurationItems
}

func (w *configurationWorkflow) moveAction(delta int) {
	w.selectedIndex = render.ClampSelectedIndex(w.selectedIndex+delta, len(w.actions))
}

func (w *configurationWorkflow) moveItem(delta int) {
	w.itemSelectedIndex = render.ClampSelectedIndex(w.itemSelectedIndex+delta, len(w.items))
}

func (w *configurationWorkflow) moveSubitem(delta int) {
	w.itemSelectedIndex = render.ClampSelectedIndex(w.itemSelectedIndex+delta, len(w.subitems))
}

func (w *configurationWorkflow) toggleSubitem() {
	idx := render.ClampSelectedIndex(w.itemSelectedIndex, len(w.subitems))
	if idx >= len(w.subitems) {
		return
	}
	w.subitems[idx].Selected = !w.subitems[idx].Selected
	if idx+1 < len(w.subitems) {
		w.itemSelectedIndex = idx + 1
	}
}

func (w *configurationWorkflow) selectAction(cwd string) View {
	idx := render.ClampSelectedIndex(w.selectedIndex, len(w.actions))
	if idx >= len(w.actions) {
		return ViewConfigurations
	}
	action := w.actions[idx]
	w.activeAction = &action
	w.itemSelectedIndex = 0

	if action.DirectMultiSelect != nil {
		return w.openDirectMultiSelect(action, cwd)
	}

	if action.Select != nil && action.Select.LoadItems != nil {
		ctx := w.context(cwd)
		items, err := action.Select.LoadItems(ctx)
		if err != nil {
			w.items = nil
			w.status = err.Error()
		} else {
			w.items = items
			w.itemSelectedIndex = selectedConfigItemIndex(items)
			w.status = ""
		}
	}
	return ViewConfigurationItems
}

func (w *configurationWorkflow) openDirectMultiSelect(action provider.ConfigAction, cwd string) View {
	item := action.DirectMultiSelect.Item
	w.activeItem = &item
	w.activeSubitems = &action.DirectMultiSelect.Subitems
	ctx := w.context(cwd)
	subitems, err := action.DirectMultiSelect.Subitems.LoadItems(item, ctx)
	if err != nil {
		w.subitems = nil
		w.status = err.Error()
		return ViewConfigurations
	}
	if len(subitems) == 0 {
		w.subitems = nil
		w.status = action.DirectMultiSelect.Subitems.EmptyMessage
		if w.status == "" {
			w.status = "No models."
		}
		return ViewConfigurations
	}
	w.subitems = subitems
	w.status = ""
	return ViewConfigurationSubitems
}

func (w *configurationWorkflow) selectItem(cwd string) View {
	if w.activeAction == nil || w.activeAction.Select == nil {
		return ViewConfigurationItems
	}
	idx := render.ClampSelectedIndex(w.itemSelectedIndex, len(w.items))
	if idx >= len(w.items) {
		return ViewConfigurationItems
	}
	item := w.items[idx]

	if w.activeAction.Select.MultiSelect != nil {
		return w.openSelectMultiSelect(item, cwd)
	}

	if w.activeAction.Select.ApplyItem != nil {
		ctx := w.context(cwd)
		status, err := w.activeAction.Select.ApplyItem(item, ctx)
		if err != nil {
			w.status = err.Error()
		} else {
			w.status = status
		}
		w.resetActive()
		return ViewConfigurations
	}
	return ViewConfigurationItems
}

func (w *configurationWorkflow) openSelectMultiSelect(item provider.ConfigItem, cwd string) View {
	w.activeItem = &item
	w.activeSubitems = w.activeAction.Select.MultiSelect
	w.itemSelectedIndex = 0
	ctx := w.context(cwd)
	subitems, err := w.activeAction.Select.MultiSelect.LoadItems(item, ctx)
	if err != nil {
		w.subitems = nil
		w.status = err.Error()
		return ViewConfigurationItems
	}
	if len(subitems) == 0 {
		w.subitems = nil
		w.status = w.activeAction.Select.MultiSelect.EmptyMessage
		if w.status == "" {
			w.status = "No models."
		}
		return ViewConfigurationItems
	}
	w.subitems = subitems
	w.status = ""
	return ViewConfigurationSubitems
}

func (w *configurationWorkflow) applySubitems(cwd string) View {
	if w.activeAction == nil || w.activeItem == nil || w.activeSubitems == nil || w.activeSubitems.Apply == nil {
		return ViewConfigurationSubitems
	}
	var selected []provider.ConfigItem
	for _, item := range w.subitems {
		if item.Selected {
			selected = append(selected, item)
		}
	}
	ctx := w.context(cwd)
	status, err := w.activeSubitems.Apply(*w.activeItem, selected, ctx)
	if err != nil {
		w.status = err.Error()
	} else {
		w.status = status
		w.actions = w.provider.ConfigurationActions()
	}
	w.resetActive()
	w.selectedIndex = render.ClampSelectedIndex(w.selectedIndex, len(w.actions))
	return ViewConfigurations
}

func (w configurationWorkflow) context(cwd string) provider.Context {
	return provider.Context{Cwd: cwd, DataHome: w.provider.DefaultHome()}
}

func selectedConfigItemIndex(items []provider.ConfigItem) int {
	for i, item := range items {
		if item.Selected {
			return i
		}
	}
	return 0
}
