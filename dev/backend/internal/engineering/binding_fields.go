package engineering

import (
	"keel.local/dev/internal/bom"
	"strings"
)

// Region capabilities mirror the fields actually consumed by each extractor.
// Known vocabulary alone must not allow, for example, cable labels in Power.
func RegionFields(kind string) []string {
	placement := "label site room rack start_u height_u mount_face depth_mm"
	facility := "label site room rack capacity_u width_mm height_mm depth_mm x_mm y_mm z_mm orientation_degrees coordinate_frame"
	power := "device label site room rack power power_unit power_basis cooling_type power_capacity power_capacity_unit cooling_capacity cooling_capacity_unit"
	cable := "a_device a_port z_device z_port a_parent_port z_parent_port a_optic z_optic patch_device patch_port patch_exit_port role rate_bps medium cable_label cable_model connector_a connector_z length length_unit length_state standard_length standard_length_unit route_from route_to distance distance_unit segment_index"
	var fields string
	switch kind {
	case "equipment":
		fields = "label equipment_type model quantity identity_kind serial role " + placement + " " + power
	case "assembly":
		fields = "parent child relation_type quantity slot"
	case "site", "room", "rack":
		fields = facility + " " + power
	case "placement":
		fields = placement
	case "port":
		fields = "device port parent_port breakout_index role rate_bps medium"
	case "connection", "cable":
		fields = cable
	case "power":
		fields = power
	case "material":
		fields = "label model required_quantity purchased_quantity spare_quantity purchase_gap quantity_unit standard_length standard_length_unit"
	case "workload":
		fields = "label model work_type work_quantity work_unit standard_length standard_length_unit labor_hours labor_unit"
	default:
		return BindingFields()
	}
	return strings.Fields(fields)
}
func validRegionField(kind, field string) bool {
	if strings.HasPrefix(field, "attribute:") {
		return validBindingField(field)
	}
	for _, f := range RegionFields(kind) {
		if f == field {
			return true
		}
	}
	return false
}

// Reuse existing generic header knowledge as hints, never as approved mappings.
// Unknown customer wording remains for the Agent to interpret from source.
func HeaderFields(value string) []string {
	field := bom.Field(value)
	aliases := map[string][]string{
		"hostname": {"label", "device"}, "device_alias": {"label", "device"}, "name": {"label"},
		"endpoint_a_device": {"a_device"}, "endpoint_b_device": {"z_device"}, "endpoint_a_port": {"a_port"}, "endpoint_b_port": {"z_port"},
		"endpoint_a_optic": {"a_optic"}, "endpoint_b_optic": {"z_optic"}, "u_start": {"start_u"}, "u_height": {"height_u"}, "rack_height": {"capacity_u"},
		"model": {"model", "cable_model"}, "quantity": {"quantity", "required_quantity", "work_quantity"}, "quantity_required": {"required_quantity"}, "quantity_ordered": {"purchased_quantity"}, "quantity_spares": {"spare_quantity"},
		"speed": {"rate_bps"}, "media_type": {"medium"}, "network_role": {"role"},
	}
	if fields, ok := aliases[field]; ok {
		return fields
	}
	if bindingFields[field] {
		return []string{field}
	}
	return nil
}
