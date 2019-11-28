package toolbox

import (
	"testing"
	"time"
)

func TestAtLeast(t *testing.T) {
	if i := AtLeast(0, -1); i != 0 {
		t.Fatal(i)
	}
	if i := AtLeast(-2, -1); i != -1 {
		t.Fatal(i)
	}
	if i := AtLeast(-2, 1); i != 1 {
		t.Fatal(i)
	}
	if i := AtLeast(0, 1); i != 1 {
		t.Fatal(i)
	}
	if i := AtLeast(1, 1); i != 1 {
		t.Fatal(i)
	}
	if i := AtLeast(3, 1); i != 3 {
		t.Fatal(i)
	}
}

func TestWolframAlpha_Execute(t *testing.T) {
	if !TestWolframAlpha.IsConfigured() {
		t.Skip("wolframalpha is not configured")
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
	} else {
		t.Log(ret.CombinedOutput)
	}
	// The timeout must be strictly obeyed
	start := time.Now().Unix()
	if ret := TestWolframAlpha.Execute(Command{TimeoutSec: 5, Content: "weather in Helsinki"}); ret.Error != nil || len(ret.ResetCombinedText()) < 5 {
		t.Fatal(ret.Error, ret.ResetCombinedText())
	} else {
		t.Log(ret.CombinedOutput)
	}
	end := time.Now().Unix()
	if end-start > 5 {
		t.Fatal("did not obey timeout")
	}
}

func TestWolframAlpha_ExtractResponse(t *testing.T) {
	input := `
<?xml version='1.0' encoding='UTF-8'?>
<queryresult success='true'
    error='false'
    numpods='7'
    datatypes='Airport,City,Star,WeatherStation'
    timedout=''
    timedoutpods=''
    timing='6.654'
    parsetiming='0.606'
    parsetimedout='false'
    recalculate=''
    id='MSP6341hfc98ahf413ha8800001hc0dacabbigc35f'
    host='https://www5b.wolframalpha.com'
    server='44'
    related='https://www5b.wolframalpha.com/api/v1/relatedQueries.jsp?id=MSPa6351hfc98ahf413ha8800002i31ei2ia816h3a3435888006500396608'
    version='2.6'>
 <pod title='Input interpretation'
     scanner='Identity'
     id='Input'
     position='100'
     error='false'
     numsubpods='1'>
  <subpod title=''>
   <plaintext>weather | Helsinki, Finland</plaintext>
  </subpod>
  <expressiontypes count='1'>
   <expressiontype name='Default' />
  </expressiontypes>
 </pod>
 <pod title='Latest recorded weather for Helsinki, Finland'
     scanner='Data'
     id='InstantaneousWeather:WeatherData'
     position='200'
     error='false'
     numsubpods='1'
     primary='true'>
  <subpod title=''>
   <microsources>
    <microsource>WeatherData</microsource>
   </microsources>
   <plaintext>temperature | 4 °C (wind chill: 0 °C)
conditions | rain, cloudy
relative humidity | 93% (dew point: 3 °C)
wind speed | 6.2 m/s
(32 minutes ago)</plaintext>
  </subpod>
  <expressiontypes count='1'>
   <expressiontype name='Grid' />
  </expressiontypes>
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
    </units>
   </info>
  </infos>
 </pod>
 <pod title='Weather forecast for Helsinki, Finland'
     scanner='Data'
     id='WeatherForecast:WeatherData'
     position='300'
     error='false'
     numsubpods='2'
     primary='true'>
  <subpod title='Tonight'>
   <microsources>
    <microsource>WeatherForecastData</microsource>
   </microsources>
   <plaintext>between 4 °C and 6 °C
rain (late afternoon to evening | late night to very early morning | early morning) | clear (all night)</plaintext>
  </subpod>
  <subpod title='Tomorrow'>
   <microsources>
    <microsource>WeatherForecastData</microsource>
   </microsources>
   <plaintext>6 °C
clear (all day) | rain (late morning onward)</plaintext>
  </subpod>
  <expressiontypes count='2'>
   <expressiontype name='Grid' />
   <expressiontype name='Grid' />
  </expressiontypes>
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
   <microsources>
    <microsource>WeatherForecastData</microsource>
   </microsources>
   <plaintext>
  | | |
low: -1 °C
Sat, Nov 30, 5:00am | average high: | 3 °C
average low: | 0 °C | high: 6 °C
Fri, Nov 29, 2:00pm
 | |  </plaintext>
  </subpod>
  <subpod title='Cloud cover'>
   <microsources>
    <microsource>WeatherForecastData</microsource>
   </microsources>
   <plaintext>
 | clear: 49.3% (2.9 days) | overcast: 18% (1.1 days) </plaintext>
  </subpod>
  <subpod title='Conditions'>
   <microsources>
    <microsource>WeatherForecastData</microsource>
   </microsources>
   <plaintext>
 | rain: 37.4% (2.2 days) | snow: 4.2% (6 hours) | fog: 3.9% (5.5 hours) </plaintext>
  </subpod>
  <subpod title='Precipitation rate'>
   <microsources>
    <microsource>WeatherForecastData</microsource>
   </microsources>
   <plaintext>
  |
high: 0.72 mm/h
Fri, Nov 29, 8:00pm, ...
</plaintext>
  </subpod>
  <expressiontypes count='4'>
   <expressiontype name='Default' />
   <expressiontype name='Grid' />
   <expressiontype name='Grid' />
   <expressiontype name='Default' />
  </expressiontypes>
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
 <pod title='Historical temperatures for November 28'
     scanner='Data'
     id='HistoricalTemperature:WeatherData'
     position='500'
     error='false'
     numsubpods='1'>
  <subpod title=''>
   <microsources>
    <microsource>CityData</microsource>
    <microsource>WeatherData</microsource>
   </microsources>
   <plaintext>
low: -21 °C
2010 | average high: | 1 °C
average low: | -4 °C | high: 9 °C
2011
(daily ranges, not corrected for changes in local weather station environment)</plaintext>
  </subpod>
  <expressiontypes count='1'>
   <expressiontype name='Grid' />
  </expressiontypes>
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
   <microsources>
    <microsource>AirportData</microsource>
    <microsource>StarData</microsource>
    <microsource>WeatherData</microsource>
   </microsources>
   <datasources>
    <datasource>PlanetaryTheoriesInRectangularAndSphericalVariables</datasource>
   </datasources>
   <plaintext>name | EFHK (Helsinki-Vantaa Airport)
relative position | 16 km N (from center of Helsinki)
relative elevation | (comparable to center of Helsinki)
local time | 8:52:06 pm EET | Thursday, November 28, 2019
local sunlight | sun is below the horizon
azimuth: 296° (WNW) | altitude: -38° (below horizon)</plaintext>
  </subpod>
  <expressiontypes count='1'>
   <expressiontype name='Grid' />
  </expressiontypes>
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
    </units>
   </info>
   <info>
    <link url='http://maps.google.com?ie=UTF8&amp;z=12&amp;t=k&amp;ll=60.317%2C24.963&amp;q=60.317%20N%2C%2024.963%20E'
        text='Satellite image' />
   </info>
  </infos>
 </pod>
 <pod title='Weather station comparisons'
     scanner='Data'
     id='LocalTemperature:WeatherData'
     position='700'
     error='false'
     numsubpods='1'>
  <subpod title=''>
   <microsources>
    <microsource>WeatherData</microsource>
   </microsources>
   <plaintext> | position | elevation | current temperature
EFHK | 16 km N | 56 m | 4 °C (32 minutes ago)
EETN | 85 km S | 40 m | 2 °C (32 minutes ago)
EFTU | 153 km WNW | 59 m | 6 °C (32 minutes ago)
(sorted by distance and inferred reliability)</plaintext>
  </subpod>
  <expressiontypes count='1'>
   <expressiontype name='Grid' />
  </expressiontypes>
  <states count='2'>
   <state name='Show non-metric'
       input='LocalTemperature:WeatherData__Show non-metric' />
   <state name='More'
       input='LocalTemperature:WeatherData__More' />
  </states>
  <infos count='1'>
   <info>
    <units count='2'>
     <unit short='km'
         long='kilometers' />
     <unit short='m'
         long='meters' />
    </units>
   </info>
  </infos>
 </pod>
 <assumptions count='1'>
  <assumption type='Clash'
      word='in'
      template='Assuming &quot;${word}&quot; is ${desc1}. Use as ${desc2} instead'
      count='2'>
   <value name='EnglishWord'
       desc='a word'
       input='*C.in-_*EnglishWord-' />
   <value name='USState'
       desc='a US state'
       input='*C.in-_*USState-' />
  </assumption>
 </assumptions>
 <userinfoused count='1'>
  <userinfo name='Country' />
 </userinfoused>
 <sources count='5'>
  <source url='https://www5b.wolframalpha.com/sources/AirportDataSourceInformationNotes.html'
      text='Airport data' />
  <source url='https://www5b.wolframalpha.com/sources/CityDataSourceInformationNotes.html'
      text='City data' />
  <source url='https://www5b.wolframalpha.com/sources/StarDataSourceInformationNotes.html'
      text='Star data' />
  <source url='https://www5b.wolframalpha.com/sources/WeatherDataSourceInformationNotes.html'
      text='Weather data' />
  <source url='https://www5b.wolframalpha.com/sources/WeatherForecastDataSourceInformationNotes.html'
      text='Weather forecast data' />
 </sources>
</queryresult>
`
	expectedOutput := `(INPUT INTERPRETATION) weather Helsinki, Finland

(LATEST RECORDED WEATHER FOR HELSINKI, FINLAND) temperature 4 °C (wind chill: 0 °C)
conditions rain, cloudy
relative humidity 93% (dew point: 3 °C)
wind speed 6.2 m/s
(32 minutes ago)

(WEATHER FORECAST FOR HELSINKI, FINLAND) [WEATHER FORECAST FOR HELSINKI, FINLAND] between 4 °C and 6 °C
rain (late afternoon to evening late night to very early morning early morning) clear (all night)
[WEATHER FORECAST FOR HELSINKI, FINLAND] 6 °C
clear (all day) rain (late morning onward)

(WEATHER HISTORY & FORECAST) [WEATHER HISTORY & FORECAST] low: -1 °C
Sat, Nov 30, 5:00am average high: 3 °C
average low: 0 °C high: 6 °C
Fri, Nov 29, 2:00pm
[WEATHER HISTORY & FORECAST] clear: 49.3% (2.9 days) overcast: 18% (1.1 days)
[WEATHER HISTORY & FORECAST] rain: 37.4% (2.2 days) snow: 4.2% (6 hours) fog: 3.9% (5.5 hours)
[WEATHER HISTORY & FORECAST] high: 0.72 mm/h
Fri, Nov 29, 8:00pm, ...

(HISTORICAL TEMPERATURES FOR NOVEMBER 28) low: -21 °C
2010 average high: 1 °C
average low: -4 °C high: 9 °C
2011
(daily ranges, not corrected for changes in local weather station environment)

(WEATHER STATION INFORMATION) name EFHK (Helsinki-Vantaa Airport)
relative position 16 km N (from center of Helsinki)
relative elevation (comparable to center of Helsinki)
local time 8:52:06 pm EET Thursday, November 28, 2019
local sunlight sun is below the horizon
azimuth: 296° (WNW) altitude: -38° (below horizon)

(WEATHER STATION COMPARISONS) position elevation current temperature
EFHK 16 km N 56 m 4 °C (32 minutes ago)
EETN 85 km S 40 m 2 °C (32 minutes ago)
EFTU 153 km WNW 59 m 6 °C (32 minutes ago)
(sorted by distance and inferred reliability)

`
	output, err := TestWolframAlpha.ExtractResponse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if output != expectedOutput {
		t.Fatalf("\n========%s========\n\n========%s========\n", output, expectedOutput)
	}
}
