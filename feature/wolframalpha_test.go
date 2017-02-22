package feature

import "testing"

func TestWolframAlpha_Execute(t *testing.T) {
	if !TestWolframAlpha.IsConfigured() {
		t.Skip()
	}
	if err := TestWolframAlpha.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := TestWolframAlpha.SelfTest(); err != nil {
		t.Fatal(err)
	}
	if ret := TestWolframAlpha.Execute(Command{TimeoutSec: 30, Content: "  "}); ret.Error == nil || ret.Error != ErrEmptyCommand {
		t.Fatal(ret)
	}
	if ret := TestWolframAlpha.Execute(Command{TimeoutSec: 30, Content: "pi"}); ret.Error != nil || len(ret.ResetCombinedText()) < 100 {
		t.Fatal(ret.Error, ret.ResetCombinedText())
	}
}

func TestWolframAlpha_ExtractResponse(t *testing.T) {
	input := `<?xml version='1.0' encoding='UTF-8'?>
<queryresult success='true'
    error='false'
    numpods='6'
    datatypes='Weather'
    timedout='Data,Character'
    timedoutpods=''
    timing='7.258'
    parsetiming='0.14'
    parsetimedout='false'
    recalculate='http://www3.wolframalpha.com/api/v2/recalc.jsp?id=MSPa5161cf6a34b84i1870c00006aif8bg164886379&amp;s=34'
    id='MSPa5171cf6a34b84i1870c00002436dcaeaa7734c2'
    host='http://www3.wolframalpha.com'
    server='34'
    related='http://www3.wolframalpha.com/api/v2/relatedQueries.jsp?id=MSPa5181cf6a34b84i1870c00001bg9f0545940f532&amp;s=34'
    version='2.6'
    profile='EnterDoQuery:0.,StartWrap:7.25811'>
 <pod title='Input interpretation'
     scanner='Identity'
     id='Input'
     position='100'
     error='false'
     numsubpods='1'>
  <subpod title=''>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5191cf6a34b84i1870c00002cb1121i5a50518e?MSPStoreType=image/gif&amp;s=34'
       alt='weather | Nuremberg, Germany'
       title='weather | Nuremberg, Germany'
       width='256'
       height='32' />
   <plaintext>weather | Nuremberg, Germany</plaintext>
  </subpod>
 </pod>
 <pod title='Latest recorded weather for Nuremberg, Germany'
     scanner='Data'
     id='InstantaneousWeather:WeatherData'
     position='200'
     error='false'
     numsubpods='1'
     primary='true'>
  <subpod title=''>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5201cf6a34b84i1870c000060i6630e77b0ccd3?MSPStoreType=image/gif&amp;s=34'
       alt='temperature | 9 °C
conditions | clear
relative humidity | 57%  (dew point: 1 °C)
wind speed | 0.5 m/s
(42 minutes ago)'
       title='temperature | 9 °C
conditions | clear
relative humidity | 57%  (dew point: 1 °C)
wind speed | 0.5 m/s
(42 minutes ago)'
       width='304'
       height='153' />
   <plaintext>temperature | 9 °C
conditions | clear
relative humidity | 57%  (dew point: 1 °C)
wind speed | 0.5 m/s
(42 minutes ago)</plaintext>
  </subpod>
  <states count='2'>
   <state name='Show non-metric'
       input='InstantaneousWeather:WeatherData__Show non-metric' />
   <state name='More'
       input='InstantaneousWeather:WeatherData__More' />
  </states>
  <infos count='1'>
   <info>
    <units count='1'>
     <unit short='m/s'
         long='meters per second' />
     <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5211cf6a34b84i1870c00002ddgceh366df6i5a?MSPStoreType=image/gif&amp;s=34'
         width='166'
         height='26' />
    </units>
   </info>
  </infos>
 </pod>
 <pod title='Weather forecast for Nuremberg, Germany'
     scanner='Data'
     id='WeatherForecast:WeatherData'
     position='300'
     error='false'
     numsubpods='2'
     primary='true'>
  <subpod title='Tonight'>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5221cf6a34b84i1870c000047a2ae5781h4ef08?MSPStoreType=image/gif&amp;s=34'
       alt='between 3 °C and 7 °C
clear (all night)'
       title='between 3 °C and 7 °C
clear (all night)'
       width='180'
       height='60' />
   <plaintext>between 3 °C and 7 °C
clear (all night)</plaintext>
  </subpod>
  <subpod title='Tomorrow'>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5231cf6a34b84i1870c00006963303a6493ce54?MSPStoreType=image/gif&amp;s=34'
       alt='between 5 °C and 14 °C
clear (all day)  |  rain (mid-morning to late afternoon)'
       title='between 5 °C and 14 °C
clear (all day)  |  rain (mid-morning to late afternoon)'
       width='391'
       height='64' />
   <plaintext>between 5 °C and 14 °C
clear (all day)  |  rain (mid-morning to late afternoon)</plaintext>
  </subpod>
  <states count='3'>
   <state name='Show non-metric'
       input='WeatherForecast:WeatherData__Show non-metric' />
   <state name='More days'
       input='WeatherForecast:WeatherData__More days' />
   <state name='More details'
       input='WeatherForecast:WeatherData__More details' />
  </states>
 </pod>
 <pod title='Weather history &amp; forecast'
     scanner='Data'
     id='WeatherCharts:WeatherData'
     position='400'
     error='false'
     numsubpods='4'>
  <subpod title='Temperature'>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5241cf6a34b84i1870c00003gi90h59haffaib7?MSPStoreType=image/gif&amp;s=34'
       alt='
  |  |   |
low: 3 °C
Sun, Apr 10, 5:00am, ... | average high:  | 14 °C
average low:  | 6 °C | high: 18 °C
Mon, Apr 11, 5:00pm
 |   |  '
       title='
  |  |   |
low: 3 °C
Sun, Apr 10, 5:00am, ... | average high:  | 14 °C
average low:  | 6 °C | high: 18 °C
Mon, Apr 11, 5:00pm
 |   |  '
       width='519'
       height='190' />
   <plaintext>
  |  |   |
low: 3 °C
Sun, Apr 10, 5:00am, ... | average high:  | 14 °C
average low:  | 6 °C | high: 18 °C
Mon, Apr 11, 5:00pm
 |   |  </plaintext>
  </subpod>
  <subpod title='Cloud cover'>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5251cf6a34b84i1870c00001f7ff4cd70hebibf?MSPStoreType=image/gif&amp;s=34'
       alt='
 | clear: 79.8% (4.7 days)   |  overcast: 4.2% (6 hours) '
       title='
 | clear: 79.8% (4.7 days)   |  overcast: 4.2% (6 hours) '
       width='519'
       height='118' />
   <plaintext>
 | clear: 79.8% (4.7 days)   |  overcast: 4.2% (6 hours) </plaintext>
  </subpod>
  <subpod title='Conditions'>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5261cf6a34b84i1870c00002da7dbc02ff09i6b?MSPStoreType=image/gif&amp;s=34'
       alt='
 | rain: 13.7% (19.5 hours) '
       title='
 | rain: 13.7% (19.5 hours) '
       width='519'
       height='87' />
   <plaintext>
 | rain: 13.7% (19.5 hours) </plaintext>
  </subpod>
  <subpod title='Precipitation rate'>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5271cf6a34b84i1870c00004b75ahed9afb0ei7?MSPStoreType=image/gif&amp;s=34'
       alt='
  |
maximum: 0.01 mm/h
Sun, Apr 10, 2:00pm
'
       title='
  |
maximum: 0.01 mm/h
Sun, Apr 10, 2:00pm
'
       width='519'
       height='156' />
   <plaintext>
  |
maximum: 0.01 mm/h
Sun, Apr 10, 2:00pm
</plaintext>
  </subpod>
  <states count='3'>
   <statelist count='9'
       value='Current week'
       delimiters=''>
    <state name='Current week'
        input='WeatherCharts:WeatherData__Current week' />
    <state name='Current day'
        input='WeatherCharts:WeatherData__Current day' />
    <state name='Next week'
        input='WeatherCharts:WeatherData__Next week' />
    <state name='Past week'
        input='WeatherCharts:WeatherData__Past week' />
    <state name='Past month'
        input='WeatherCharts:WeatherData__Past month' />
    <state name='Past year'
        input='WeatherCharts:WeatherData__Past year' />
    <state name='Past 5 years'
        input='WeatherCharts:WeatherData__Past 5 years' />
    <state name='Past 10 years'
        input='WeatherCharts:WeatherData__Past 10 years' />
    <state name='All'
        input='WeatherCharts:WeatherData__All' />
   </statelist>
   <state name='Show non-metric'
       input='WeatherCharts:WeatherData__Show non-metric' />
   <state name='More'
       input='WeatherCharts:WeatherData__More' />
  </states>
 </pod>
 <pod title='Historical temperatures for April 8'
     scanner='Data'
     id='HistoricalTemperature:WeatherData'
     position='500'
     error='false'
     numsubpods='1'>
  <subpod title=''>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5281cf6a34b84i1870c00004i6f39ibhd81b098?MSPStoreType=image/gif&amp;s=34'
       alt='
low: -10 °C
2003 | average high:  | 12 °C
average low:  | 1 °C | high: 21 °C
1986
(daily ranges, not corrected for changes in local weather station environment)'
       title='
low: -10 °C
2003 | average high:  | 12 °C
average low:  | 1 °C | high: 21 °C
1986
(daily ranges, not corrected for changes in local weather station environment)'
       width='519'
       height='161' />
   <plaintext>
low: -10 °C
2003 | average high:  | 12 °C
average low:  | 1 °C | high: 21 °C
1986
(daily ranges, not corrected for changes in local weather station environment)</plaintext>
  </subpod>
  <states count='2'>
   <state name='Show table'
       input='HistoricalTemperature:WeatherData__Show table' />
   <state name='Show non-metric'
       input='HistoricalTemperature:WeatherData__Show non-metric' />
  </states>
 </pod>
 <pod title='Weather station information'
     scanner='Data'
     id='WeatherStationInformation:WeatherData'
     position='600'
     error='false'
     numsubpods='1'>
  <subpod title=''>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5291cf6a34b84i1870c00004fhgi340c08c233e?MSPStoreType=image/gif&amp;s=34'
       alt='name | EDDN  (Nürnberg Airport)
relative position | 6 km N  (from center of Nuremberg)
relative elevation | (comparable to center of Nuremberg)
local time | 9:02:24 pm CEST  |  Friday, April 8, 2016
local sunlight | sun is below the horizon
azimuth: 295°  (WNW)  |  altitude: -10°'
       title='name | EDDN  (Nürnberg Airport)
relative position | 6 km N  (from center of Nuremberg)
relative elevation | (comparable to center of Nuremberg)
local time | 9:02:24 pm CEST  |  Friday, April 8, 2016
local sunlight | sun is below the horizon
azimuth: 295°  (WNW)  |  altitude: -10°'
       width='435'
       height='185' />
   <plaintext>name | EDDN  (Nürnberg Airport)
relative position | 6 km N  (from center of Nuremberg)
relative elevation | (comparable to center of Nuremberg)
local time | 9:02:24 pm CEST  |  Friday, April 8, 2016
local sunlight | sun is below the horizon
azimuth: 295°  (WNW)  |  altitude: -10°</plaintext>
  </subpod>
  <states count='2'>
   <state name='Show non-metric'
       input='WeatherStationInformation:WeatherData__Show non-metric' />
   <state name='More'
       input='WeatherStationInformation:WeatherData__More' />
  </states>
  <infos count='2'>
   <info>
    <units count='1'>
     <unit short='km'
         long='kilometers' />
     <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5301cf6a34b84i1870c00003ec8cc16181cg7id?MSPStoreType=image/gif&amp;s=34'
         width='118'
         height='26' />
    </units>
   </info>
   <info>
    <link url='http://maps.google.com?ie=UTF8&amp;z=12&amp;t=k&amp;ll=49.503%2C11.055&amp;q=49.503%20N%2C%2011.055%20E'
        text='Satellite image' />
   </info>
  </infos>
 </pod>
 <sources count='5'>
  <source url='http://www.wolframalpha.com/sources/AirportDataSourceInformationNotes.html'
      text='Airport data' />
  <source url='http://www.wolframalpha.com/sources/AstronomicalDataSourceInformationNotes.html'
      text='Astronomical data' />
  <source url='http://www.wolframalpha.com/sources/CityDataSourceInformationNotes.html'
      text='City data' />
  <source url='http://www.wolframalpha.com/sources/WeatherDataSourceInformationNotes.html'
      text='Weather data' />
  <source url='http://www.wolframalpha.com/sources/WeatherForecastDataSourceInformationNotes.html'
      text='Weather forecast data' />
 </sources>
</queryresult>`
	txtInfo, err := TestWolframAlpha.ExtractResponse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if txtInfo != `weather Nuremberg, Germany.temperature 9 °C
conditions clear
relative humidity 57%  (dew point: 1 °C)
wind speed 0.5 m/s
(42 minutes ago).between 3 °C and 7 °C
clear (all night).between 5 °C and 14 °C
clear (all day)   rain (mid-morning to late afternoon).low: 3 °C
Sun, Apr 10, 5:00am, ... average high:  14 °C
average low:  6 °C high: 18 °C
Mon, Apr 11, 5:00pm.clear: 79.8% (4.7 days)    overcast: 4.2% (6 hours).rain: 13.7% (19.5 hours).maximum: 0.01 mm/h
Sun, Apr 10, 2:00pm.low: -10 °C
2003 average high:  12 °C
average low:  1 °C high: 21 °C
1986
(daily ranges, not corrected for changes in local weather station environment).name EDDN  (Nürnberg Airport)
relative position 6 km N  (from center of Nuremberg)
relative elevation (comparable to center of Nuremberg)
local time 9:02:24 pm CEST   Friday, April 8, 2016
local sunlight sun is below the horizon
azimuth: 295°  (WNW)   altitude: -10°.` {
		t.Fatal(txtInfo)
	}
}
