import type {App} from 'vue';
import KsSplitter from '../../vendor/kestra/design-system/components/Basic/KsSplitter/KsSplitter.vue';
import KsSplitterPanel from '../../vendor/kestra/design-system/components/Basic/KsSplitter/KsSplitterPanel.vue';
import KsButton from '../../vendor/kestra/design-system/components/Basic/KsButton/KsButton.vue';
import KsTooltip from '../../vendor/kestra/design-system/components/Feedback/KsTooltip.vue';
import KsNoData from '../../vendor/kestra/design-system/components/Data/KsNoData.vue';
import KsDropdown from '../../vendor/kestra/design-system/components/Navigation/KsDropdown/KsDropdown.vue';
import KsDropdownMenu from '../../vendor/kestra/design-system/components/Navigation/KsDropdown/KsDropdownMenu.vue';
import KsDropdownItem from '../../vendor/kestra/design-system/components/Navigation/KsDropdown/KsDropdownItem.vue';
export function registerNativePanels(app:App){for(const [key,component] of Object.entries({KsSplitter,KsSplitterPanel,KsButton,KsTooltip,KsNoData,KsDropdown,KsDropdownMenu,KsDropdownItem}))app.component(key,component)}
