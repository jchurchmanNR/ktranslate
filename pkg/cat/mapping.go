package cat

import (
	"bufio"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/kentik/ktranslate/pkg/kt"
)

// List of ids to name mapping.
// Load per region with ch_www=> select '"' || id || '": "' || col_name || '",' from  mn_kflow_field where is_public = true and status = 'A' and col_name not like 'i_%';
/*
 Load interfaces for a company with:

with t as (SELECT
   -- device fields
   d.id AS device_id,
   d.device_name,
   d.device_type,
   COALESCE(d.site_id, 0) AS site_id,
   -- interface fields
   COALESCE(i.snmp_id, '') AS snmp_id,
   COALESCE(i.snmp_speed, 0) AS snmp_speed,
   COALESCE(i.snmp_type, 0) AS snmp_type,
   COALESCE(i.snmp_alias, '') AS snmp_alias,
   COALESCE(i.interface_ip, '127.0.0.1') AS interface_ip,
   COALESCE(i.interface_description, '') AS interface_description,
   COALESCE(i.provider, '') AS provider,
   i.vrf_id as vrf_id,
   -- site fields
   COALESCE(s.title, '') AS site_title,
   COALESCE(s.country, '') AS site_country
  FROM mn_device AS d
  LEFT JOIN mn_interface AS i ON (d.id = i.device_id) AND (d.company_id = i.company_id)
  LEFT JOIN mn_site AS s ON (d.site_id = s.id) AND (d.company_id = s.company_id)
  WHERE d.company_id = $1
) select json_agg(t) from t;

 Load UDRs with
 select app_protocol_id || ',' || custom_column || ',' || dimension_label || ',' || display_name from  mn_lookup_app_protocol as a join mn_lookup_app_protocol_cols as b on (a.id = b.app_protocol_id) order by app_protocol_id;
*/

func NewCustomMapper(file string) (*CustomMapper, error) {
	m := CustomMapper{}
	by, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(by, &m)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

func NewDeviceMapper(file string) (*DeviceMapper, int, error) {
	ms := []InterfaceRow{}
	by, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, 0, err
	}
	err = json.Unmarshal(by, &ms)
	if err != nil {
		return nil, 0, err
	}

	dm := DeviceMapper{
		Devices: make(map[kt.DeviceID]map[kt.IfaceID]*InterfaceRow),
	}

	for _, row := range ms {
		did := kt.DeviceID(row.DeviceId)
		sid, _ := strconv.Atoi(row.SnmpId)
		if _, ok := dm.Devices[did]; !ok {
			dm.Devices[did] = make(map[kt.IfaceID]*InterfaceRow)
		}
		dm.Devices[did][kt.IfaceID(sid)] = &row
	}

	return &dm, len(ms), nil
}

func NewUDRMapper(file string) (*UDRMapper, int, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)

	um := UDRMapper{
		UDRs: make(map[int32]map[string]*UDR),
	}

	found := 0
	for scanner.Scan() {
		pts := strings.SplitN(scanner.Text(), ",", 4)
		if len(pts) != 4 {
			continue
		}
		udr := UDR{
			ColumnName:      pts[2],
			ApplicationName: pts[3],
			Type:            UDR_TYPE_INT,
		}
		if strings.HasPrefix(pts[1], "STR") || strings.HasPrefix(pts[1], "INET_") {
			udr.Type = UDR_TYPE_STRING
		}
		if strings.HasPrefix(pts[1], "INT64_") {
			udr.Type = UDR_TYPE_BIGINT
		}

		appId, err := strconv.Atoi(pts[0])
		if err != nil {
			continue
		}
		app := int32(appId)
		if _, ok := um.UDRs[app]; !ok {
			um.UDRs[app] = make(map[string]*UDR)
		}
		um.UDRs[app][pts[1]] = &udr
		found++
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}

	return &um, found, nil
}